package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

type casosDePrueba struct {
	res         domain.Resultado
	err         error
	correlacion string
}

func (c *casosDePrueba) Asignar(ctx context.Context, _ domain.PedidoDeSlot) (domain.Resultado, error) {
	c.correlacion = aplicacion.CorrelacionDe(ctx)
	return c.res, c.err
}

func (c *casosDePrueba) ListarSlots(context.Context, time.Time, int64) ([]aplicacion.BloqueDelDia, error) {
	return nil, nil
}

func (c *casosDePrueba) ListarPistas(context.Context) ([]domain.Pista, error) {
	return []domain.Pista{{ID: 1, Nombre: "Pista 1", Abierta: true}}, nil
}

const cuerpoValido = `{"vuelo_id":"AR1550","tipo_operacion":"arribo","pista_id":1,"inicio_bloque":"2026-10-23T14:15:00-03:00"}`

func pedir(t *testing.T, casos CasosDeUso, cuerpo string, encabezados map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := NuevoRouter(casos, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "/asignaciones", strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range encabezados {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var json_ map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &json_)
	return rec, json_
}

// Cada error del núcleo sale con el estado y el código de docs/contracts/pistas.yaml.
func TestErroresSegunElContrato(t *testing.T) {
	casos := []struct {
		err    error
		estado int
		codigo string
	}{
		{domain.ErrDatosInvalidos{Motivo: "falta vuelo_id"}, 400, "datos_invalidos"},
		{domain.ErrPistaInexistente, 404, "pista_inexistente"},
		{domain.ErrPistaCerrada, 409, "pista_cerrada"},
		{domain.ErrVueloConSlot, 409, "vuelo_con_slot"},
		{domain.ErrSlotOcupado{}, 409, "slot_ocupado"},
		{errors.New("la base no responde"), 503, "no_disponible"},
	}
	for _, c := range casos {
		rec, cuerpo := pedir(t, &casosDePrueba{err: c.err}, cuerpoValido, nil)
		if rec.Code != c.estado || cuerpo["error"] != c.codigo {
			t.Errorf("%v: vino %d %v, se esperaba %d %s", c.err, rec.Code, cuerpo["error"], c.estado, c.codigo)
		}
		if cuerpo["mensaje"] == "" || cuerpo["mensaje"] == nil {
			t.Errorf("%v: falta el mensaje", c.err)
		}
	}
}

func TestSlotOcupadoTraeLosLibres(t *testing.T) {
	libre := domain.SlotLibre{PistaID: 2, InicioBloque: time.Date(2026, 10, 23, 14, 15, 0, 0, domain.ZonaCordoba)}
	_, cuerpo := pedir(t, &casosDePrueba{err: domain.ErrSlotOcupado{Libres: []domain.SlotLibre{libre}}}, cuerpoValido, nil)

	libres, _ := cuerpo["slots_libres"].([]any)
	if len(libres) != 1 {
		t.Fatalf("se esperaba un libre, vino %v", cuerpo)
	}
	primero := libres[0].(map[string]any)
	if primero["inicio_bloque"] != "2026-10-23T14:15:00-03:00" {
		t.Fatalf("el horario va en RFC 3339 con hora de Córdoba, vino %v", primero["inicio_bloque"])
	}
}

func TestAsignadoYRepetido(t *testing.T) {
	slot := domain.Slot{ID: 7, PistaID: 1, VueloID: "AR1550", TipoOperacion: domain.Arribo,
		InicioBloque: time.Date(2026, 10, 23, 14, 15, 0, 0, domain.ZonaCordoba)}

	rec, cuerpo := pedir(t, &casosDePrueba{res: domain.Resultado{Slot: slot, VueloDesplazadoID: "LA2000"}}, cuerpoValido, nil)
	if rec.Code != http.StatusCreated || cuerpo["vuelo_desplazado_id"] != "LA2000" || cuerpo["slot_id"] != float64(7) {
		t.Fatalf("asignado: vino %d %v", rec.Code, cuerpo)
	}

	rec, cuerpo = pedir(t, &casosDePrueba{res: domain.Resultado{Slot: slot, YaAsignado: true}}, cuerpoValido, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("repetido: se esperaba 200, vino %d", rec.Code)
	}
	if _, hay := cuerpo["vuelo_desplazado_id"]; hay {
		t.Fatalf("repetido: no tiene que traer vuelo_desplazado_id")
	}
}

func TestCuerpoMalFormado(t *testing.T) {
	rec, cuerpo := pedir(t, &casosDePrueba{}, `{"inicio_bloque":"mañana"}`, nil)
	if rec.Code != http.StatusBadRequest || cuerpo["error"] != "datos_invalidos" {
		t.Fatalf("vino %d %v", rec.Code, cuerpo)
	}
}

// 11.2: el identificador de la traza que llega en traceparent es el de correlación.
func TestTomaLaCorrelacionDelTraceparent(t *testing.T) {
	casos := &casosDePrueba{}
	pedir(t, casos, cuerpoValido, map[string]string{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	})
	if casos.correlacion != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("vino %q", casos.correlacion)
	}
}
