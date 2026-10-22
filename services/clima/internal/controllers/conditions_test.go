package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/services"
)

// fakeWeather responde siempre lo mismo, o ErrNoData si noData es true.
type fakeWeather struct{ noData bool }

func (f *fakeWeather) Airport() models.Airport { return models.Cordoba }

func (f *fakeWeather) Current(context.Context) (models.Observation, models.Operability, error) {
	if f.noData {
		return models.Observation{}, models.Operability{}, services.ErrNoData
	}
	obs := models.Observation{Airport: "SACO", ObservedAt: time.Now(), Conditions: models.Conditions{VisibilityM: 10000}}
	return obs, models.Operability{Operable: true, Reasons: []string{}}, nil
}

func (f *fakeWeather) SetForced(context.Context, services.ForcedInput) (models.Conditions, models.Operability, error) {
	return models.Conditions{}, models.Operability{}, nil
}

func (f *fakeWeather) ClearForced(context.Context) error { return nil }

func newRouter(weather WeatherReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ctl := NewConditionsController(weather)
	public := r.Group("", RequireAPIKey(map[string]string{"k1": "grupo1", "k2": "grupo2"}), RateLimit())
	public.GET("/airports/:icao/conditions", ctl.Get)
	internal := r.Group("/internal", RequireOperator())
	internal.PUT("/airports/:icao/forced-conditions", ctl.PutForced)
	return r
}

func get(r http.Handler, path, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func problemCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, se esperaba application/problem+json", ct)
	}
	var p problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("la respuesta no es un Problem Details: %s", w.Body.String())
	}
	return p.Code
}

func TestErroresDelContrato(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		key    string
		noData bool
		status int
		code   string
	}{
		{"sin clave", "/airports/SACO/conditions", "", false, 401, codeMissingAPIKey},
		{"clave inválida", "/airports/SACO/conditions", "otra", false, 401, codeInvalidAPIKey},
		{"código mal formado", "/airports/cor/conditions", "k1", false, 400, codeInvalidAirportCode},
		{"aeropuerto no atendido", "/airports/SAWH/conditions", "k1", false, 404, codeAirportNotSupported},
		{"sin dato reciente", "/airports/SACO/conditions", "k1", true, 503, codeWeatherSourceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := get(newRouter(&fakeWeather{noData: tt.noData}), tt.path, tt.key)
			if w.Code != tt.status {
				t.Fatalf("status = %d, se esperaba %d: %s", w.Code, tt.status, w.Body.String())
			}
			if code := problemCode(t, w); code != tt.code {
				t.Errorf("code = %q, se esperaba %q", code, tt.code)
			}
		})
	}
}

func TestSinDatoPideReintentarEnUnMinuto(t *testing.T) {
	w := get(newRouter(&fakeWeather{noData: true}), "/airports/SACO/conditions", "k1")
	if got := w.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, se esperaba 60", got)
	}
}

func TestRespuestaCorrecta(t *testing.T) {
	w := get(newRouter(&fakeWeather{}), "/airports/SACO/conditions", "k1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var body conditionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Airport.ICAO != "SACO" || !body.Operability.Operable {
		t.Errorf("respuesta inesperada: %+v", body)
	}
	if w.Header().Get("X-RateLimit-Remaining") != "59" {
		t.Errorf("X-RateLimit-Remaining = %q, se esperaba 59", w.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestLimiteDe60PorClave(t *testing.T) {
	r := newRouter(&fakeWeather{})
	for i := 1; i <= 60; i++ {
		if w := get(r, "/airports/SACO/conditions", "k1"); w.Code != http.StatusOK {
			t.Fatalf("el pedido %d tendría que pasar, dio %d", i, w.Code)
		}
	}
	w := get(r, "/airports/SACO/conditions", "k1")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("el pedido 61 tendría que dar 429, dio %d", w.Code)
	}
	if code := problemCode(t, w); code != codeRateLimitExceeded {
		t.Errorf("code = %q", code)
	}
	if w.Header().Get("Retry-After") != "1" {
		t.Errorf("Retry-After = %q, se esperaba 1", w.Header().Get("Retry-After"))
	}
	// Cada clave tiene su propio balde.
	if w := get(r, "/airports/SACO/conditions", "k2"); w.Code != http.StatusOK {
		t.Errorf("la otra clave no tendría que estar limitada, dio %d", w.Code)
	}
}

func TestForzadoSoloParaElOperador(t *testing.T) {
	r := newRouter(&fakeWeather{})
	req := httptest.NewRequest(http.MethodPut, "/internal/airports/SACO/forced-conditions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("sin rol de operador tendría que dar 403, dio %d", w.Code)
	}
}
