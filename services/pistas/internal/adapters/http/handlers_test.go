package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/observability"
	"github.com/gin-gonic/gin"
)

// TestMain configura OpenTelemetry como en el arranque, sin colector: así la traza que llega en
// traceparent se lee igual que en el servicio.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	if _, err := observability.Setup(context.Background(), "pistas"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type casosDePrueba struct {
	res         domain.Resultado
	err         error
	pedido      domain.PedidoDeSlot
	correlacion string
	traceparent string
}

func (c *casosDePrueba) Asignar(ctx context.Context, p domain.PedidoDeSlot) (domain.Resultado, error) {
	c.pedido = p
	c.correlacion = aplicacion.CorrelacionDe(ctx)
	c.traceparent = aplicacion.TraceParentDe(ctx)
	return c.res, c.err
}

// El ejemplo de services/pistas/openapi.yaml.
const cuerpoValido = `{"flight_id":"7f1c2a3e-5b6d-4e8f-9a0b-1c2d3e4f5a6b","flight_type":"departure","runway_id":"05-23","block_start":"2026-10-23T14:30:00Z"}`

func pedir(t *testing.T, casos CasosDeUso, cuerpo string, encabezados map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := NuevoRouter(casos, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "/internal/slots", strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range encabezados {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var cuerpoJSON map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &cuerpoJSON)
	return rec, cuerpoJSON
}

func TestLeeElPedidoDelContrato(t *testing.T) {
	casos := &casosDePrueba{}
	pedir(t, casos, cuerpoValido, nil)

	esperado := domain.PedidoDeSlot{
		VueloID:       "7f1c2a3e-5b6d-4e8f-9a0b-1c2d3e4f5a6b",
		TipoOperacion: domain.Despegue,
		PistaID:       "05-23",
		InicioBloque:  time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC),
	}
	p := casos.pedido
	if p.VueloID != esperado.VueloID || p.TipoOperacion != esperado.TipoOperacion ||
		p.PistaID != esperado.PistaID || !p.InicioBloque.Equal(esperado.InicioBloque) {
		t.Fatalf("vino %+v, se esperaba %+v", p, esperado)
	}
}

// Cada error del núcleo sale con el estado y el code del contrato, en Problem Details.
func TestErroresSegunElContrato(t *testing.T) {
	casos := []struct {
		err    error
		estado int
		codigo string
	}{
		{domain.ErrDatosInvalidos{Motivo: "falta flight_id"}, 400, "INVALID_REQUEST"},
		{domain.ErrPistaInexistente, 404, "RUNWAY_NOT_FOUND"},
		{domain.ErrPistaCerrada, 409, "RUNWAY_CLOSED"},
		{domain.ErrVueloConSlot, 409, "FLIGHT_ALREADY_HAS_SLOT"},
		{domain.ErrSlotOcupado{}, 409, "SLOT_TAKEN"},
		// Una falla pasajera es 503: Vuelos la reintenta (4.5).
		{errors.New("la base no responde"), 503, "UNAVAILABLE"},
	}
	for _, c := range casos {
		rec, cuerpo := pedir(t, &casosDePrueba{err: c.err}, cuerpoValido, nil)
		if rec.Code != c.estado || cuerpo["code"] != c.codigo || cuerpo["status"] != float64(c.estado) {
			t.Errorf("%v: vino %d %v, se esperaba %d %s", c.err, rec.Code, cuerpo, c.estado, c.codigo)
		}
		if cuerpo["type"] == nil || cuerpo["title"] == nil {
			t.Errorf("%v: faltan type o title: %v", c.err, cuerpo)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
			t.Errorf("%v: el Content-Type tiene que ser application/problem+json, vino %q", c.err, ct)
		}
	}
}

func TestSlotOcupadoTraeLosLibres(t *testing.T) {
	libre := domain.SlotLibre{PistaID: "18-36", InicioBloque: time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)}
	_, cuerpo := pedir(t, &casosDePrueba{err: domain.ErrSlotOcupado{Libres: []domain.SlotLibre{libre}}}, cuerpoValido, nil)

	libres, _ := cuerpo["nearest_free_slots"].([]any)
	if len(libres) != 1 {
		t.Fatalf("se esperaba un libre, vino %v", cuerpo)
	}
	primero := libres[0].(map[string]any)
	if primero["runway_id"] != "18-36" || primero["block_start"] != "2026-10-23T14:30:00Z" {
		t.Fatalf("vino %v", primero)
	}
}

// 4.15: responde 200 tanto al asignar como al repetir; displaced_flight_id va siempre, null si no hubo.
func TestAsignadoYRepetido(t *testing.T) {
	slot := domain.Slot{ID: 7, PistaID: "05-23", VueloID: "AR1550", TipoOperacion: domain.Arribo,
		InicioBloque: time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)}

	rec, cuerpo := pedir(t, &casosDePrueba{res: domain.Resultado{Slot: slot, VueloDesplazadoID: "LA2000"}}, cuerpoValido, nil)
	if rec.Code != http.StatusOK || cuerpo["displaced_flight_id"] != "LA2000" ||
		cuerpo["runway_id"] != "05-23" || cuerpo["block_start"] != "2026-10-23T14:30:00Z" || cuerpo["flight_id"] != "AR1550" {
		t.Fatalf("asignado: vino %d %v", rec.Code, cuerpo)
	}

	rec, cuerpo = pedir(t, &casosDePrueba{res: domain.Resultado{Slot: slot, YaAsignado: true}}, cuerpoValido, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("repetido: se esperaba 200, vino %d", rec.Code)
	}
	if v, hay := cuerpo["displaced_flight_id"]; !hay || v != nil {
		t.Fatalf("repetido: displaced_flight_id tiene que venir en null, vino %v", cuerpo)
	}
}

func TestCuerpoMalFormado(t *testing.T) {
	rec, cuerpo := pedir(t, &casosDePrueba{}, `{"block_start":"mañana"}`, nil)
	if rec.Code != http.StatusBadRequest || cuerpo["code"] != "INVALID_REQUEST" {
		t.Fatalf("vino %d %v", rec.Code, cuerpo)
	}
}

// 11.2 y 11.6: la traza que llega de Vuelos en traceparent es la del evento: su identificador va en
// el correlation_id y el traceparent se guarda para el mensaje.
func TestTomaLaCorrelacionDelTraceparent(t *testing.T) {
	casos := &casosDePrueba{}
	pedir(t, casos, cuerpoValido, map[string]string{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	})
	if casos.correlacion != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("correlación: vino %q", casos.correlacion)
	}
	if !strings.HasPrefix(casos.traceparent, "00-4bf92f3577b34da6a3ce929d0e0e4736-") {
		t.Fatalf("el traceparent tiene que seguir la misma traza, vino %q", casos.traceparent)
	}
}
