package httpadapter

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

// operadorDePrueba responde lo que se le configura en los casos de uso del operador.
type operadorDePrueba struct {
	pistas      []domain.Pista
	clima       domain.EstadoClima
	pista       domain.Pista
	alternativo domain.Alternativo
	alts        []domain.Alternativo
	err         error
	llamado     string
	argumento   string
	disponible  bool
}

func (o *operadorDePrueba) ListarPistas(context.Context) ([]domain.Pista, domain.EstadoClima, error) {
	o.llamado = "listar"
	return o.pistas, o.clima, o.err
}

func (o *operadorDePrueba) AltaDePista(_ context.Context, id string) (domain.Pista, error) {
	o.llamado, o.argumento = "alta", id
	return o.pista, o.err
}

func (o *operadorDePrueba) CerrarPista(_ context.Context, id string) (domain.Pista, error) {
	o.llamado, o.argumento = "cerrar", id
	return o.pista, o.err
}

func (o *operadorDePrueba) ReabrirPista(_ context.Context, id string) (domain.Pista, error) {
	o.llamado, o.argumento = "reabrir", id
	return o.pista, o.err
}

func (o *operadorDePrueba) ListarAlternativos(context.Context) ([]domain.Alternativo, error) {
	o.llamado = "alternativos"
	return o.alts, o.err
}

func (o *operadorDePrueba) CambiarDisponibilidad(_ context.Context, icao string, disponible bool) (domain.Alternativo, error) {
	o.llamado, o.argumento, o.disponible = "disponibilidad", icao, disponible
	return o.alternativo, o.err
}

var operador = map[string]string{"X-User-Id": "op-1", "X-User-Role": "operator"}

func pedirComoOperador(t *testing.T, casos CasosDeUso, metodo, ruta, cuerpo string, encabezados map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := NuevoRouter(casos, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
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

// CA-19 y 23.2: sin identidad, 401; con otro rol, 403. Vale para todas las rutas del operador.
func TestRutasDelOperadorPidenElRol(t *testing.T) {
	rutas := [][2]string{
		{http.MethodGet, "/runways"},
		{http.MethodPost, "/runways"},
		{http.MethodPost, "/runways/05-23/close"},
		{http.MethodPost, "/runways/05-23/reopen"},
		{http.MethodGet, "/alternate-airports"},
		{http.MethodPost, "/alternate-airports/SAOU/disable"},
		{http.MethodPost, "/alternate-airports/SAOU/enable"},
	}
	for _, r := range rutas {
		casos := &casosDePrueba{}
		rec, cuerpo := pedirComoOperador(t, casos, r[0], r[1], `{"id":"09-27"}`, nil)
		if rec.Code != http.StatusUnauthorized || cuerpo["code"] != "MISSING_IDENTITY" {
			t.Errorf("%s %s sin identidad: %d %v", r[0], r[1], rec.Code, cuerpo["code"])
		}
		rec, cuerpo = pedirComoOperador(t, casos, r[0], r[1], `{"id":"09-27"}`,
			map[string]string{"X-User-Id": "ar-1", "X-User-Role": "airline", "X-Airline": "AR"})
		if rec.Code != http.StatusForbidden || cuerpo["code"] != "FORBIDDEN_ROLE" {
			t.Errorf("%s %s como aerolínea: %d %v", r[0], r[1], rec.Code, cuerpo["code"])
		}
		if casos.llamado != "" {
			t.Errorf("%s %s: no tenía que llegar al caso de uso (%s)", r[0], r[1], casos.llamado)
		}
	}
}

// El formato de GET /runways del contrato: pistas y clima.
func TestListarPistasSegunElContrato(t *testing.T) {
	operable := false
	casos := &casosDePrueba{operadorDePrueba: operadorDePrueba{
		pistas: []domain.Pista{
			{ID: "05-23", Abierta: true},
			{ID: "18-36", Abierta: false, CerradaPor: domain.CerradaPorOperador},
		},
		clima: domain.EstadoClima{Operable: &operable, ConDatos: false},
	}}
	rec, cuerpo := pedirComoOperador(t, casos, http.MethodGet, "/runways", "", operador)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	pistas := cuerpo["runways"].([]any)
	abierta := pistas[0].(map[string]any)
	cerrada := pistas[1].(map[string]any)
	if abierta["status"] != "open" || abierta["closed_reason"] != nil {
		t.Errorf("pista abierta: %v", abierta)
	}
	if cerrada["status"] != "closed" || cerrada["closed_reason"] != "operator" {
		t.Errorf("pista cerrada: %v", cerrada)
	}
	clima := cuerpo["weather"].(map[string]any)
	if clima["data_available"] != false || clima["operable"] != false {
		t.Errorf("clima: %v", clima)
	}
}

func TestAltaDePistaPorHTTP(t *testing.T) {
	casos := &casosDePrueba{operadorDePrueba: operadorDePrueba{pista: domain.Pista{ID: "09-27", Abierta: true}}}
	rec, cuerpo := pedirComoOperador(t, casos, http.MethodPost, "/runways", `{"id":"09-27"}`, operador)
	if rec.Code != http.StatusCreated || cuerpo["id"] != "09-27" || cuerpo["status"] != "open" || casos.argumento != "09-27" {
		t.Errorf("alta: %d %v (argumento %q)", rec.Code, cuerpo, casos.argumento)
	}

	casos.operadorDePrueba.err = domain.ErrPistaYaExiste
	rec, cuerpo = pedirComoOperador(t, casos, http.MethodPost, "/runways", `{"id":"05-23"}`, operador)
	if rec.Code != http.StatusConflict || cuerpo["code"] != "RUNWAY_ALREADY_EXISTS" {
		t.Errorf("repetida: %d %v", rec.Code, cuerpo["code"])
	}

	casos.operadorDePrueba.err = nil
	rec, cuerpo = pedirComoOperador(t, casos, http.MethodPost, "/runways", `no es json`, operador)
	if rec.Code != http.StatusBadRequest || cuerpo["code"] != "INVALID_REQUEST" {
		t.Errorf("cuerpo roto: %d %v", rec.Code, cuerpo["code"])
	}
}

func TestCerrarYReabrirPorHTTP(t *testing.T) {
	casos := &casosDePrueba{operadorDePrueba: operadorDePrueba{
		pista: domain.Pista{ID: "05-23", Abierta: false, CerradaPor: domain.CerradaPorOperador},
	}}
	rec, cuerpo := pedirComoOperador(t, casos, http.MethodPost, "/runways/05-23/close", "", operador)
	if rec.Code != http.StatusOK || casos.llamado != "cerrar" || casos.argumento != "05-23" || cuerpo["closed_reason"] != "operator" {
		t.Errorf("cerrar: %d %v", rec.Code, cuerpo)
	}

	casos.operadorDePrueba.err = domain.ErrCerradaPorClima
	rec, cuerpo = pedirComoOperador(t, casos, http.MethodPost, "/runways/05-23/reopen", "", operador)
	if rec.Code != http.StatusConflict || cuerpo["code"] != "RUNWAY_CLOSED_BY_WEATHER" {
		t.Errorf("reabrir con clima: %d %v", rec.Code, cuerpo["code"])
	}

	casos.operadorDePrueba.err = domain.ErrPistaInexistente
	rec, cuerpo = pedirComoOperador(t, casos, http.MethodPost, "/runways/99-99/close", "", operador)
	if rec.Code != http.StatusNotFound || cuerpo["code"] != "RUNWAY_NOT_FOUND" {
		t.Errorf("inexistente: %d %v", rec.Code, cuerpo["code"])
	}
}

func TestAlternativosPorHTTP(t *testing.T) {
	casos := &casosDePrueba{operadorDePrueba: operadorDePrueba{
		alts:        []domain.Alternativo{{ICAO: "SAOU", Nombre: "San Luis", Disponible: true}},
		alternativo: domain.Alternativo{ICAO: "SAOU", Nombre: "San Luis", Disponible: false},
	}}
	rec, cuerpo := pedirComoOperador(t, casos, http.MethodGet, "/alternate-airports", "", operador)
	lista := cuerpo["alternate_airports"].([]any)
	if rec.Code != http.StatusOK || len(lista) != 1 || lista[0].(map[string]any)["name"] != "San Luis" {
		t.Errorf("lista: %d %v", rec.Code, cuerpo)
	}

	rec, cuerpo = pedirComoOperador(t, casos, http.MethodPost, "/alternate-airports/SAOU/disable", "", operador)
	if rec.Code != http.StatusOK || casos.disponible || cuerpo["available"] != false {
		t.Errorf("disable: %d %v", rec.Code, cuerpo)
	}
	_, _ = pedirComoOperador(t, casos, http.MethodPost, "/alternate-airports/SAOU/enable", "", operador)
	if !casos.disponible {
		t.Errorf("enable tenía que pedir disponible=true")
	}

	casos.operadorDePrueba.err = domain.ErrAlternativoInexistente
	rec, cuerpo = pedirComoOperador(t, casos, http.MethodPost, "/alternate-airports/SACO/disable", "", operador)
	if rec.Code != http.StatusNotFound || cuerpo["code"] != "ALTERNATE_NOT_FOUND" {
		t.Errorf("inexistente: %d %v", rec.Code, cuerpo["code"])
	}
}

// /internal/slots sigue sin pedir identidad: la llama Vuelos, no una persona (4.15).
func TestLaRutaInternaNoPideIdentidad(t *testing.T) {
	casos := &casosDePrueba{}
	rec, _ := pedir(t, casos, cuerpoValido, nil)
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Errorf("/internal/slots no tiene que pedir identidad: %d", rec.Code)
	}
}
