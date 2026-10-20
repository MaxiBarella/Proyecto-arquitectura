package repositories

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

// sampleResponse es una respuesta real de Open-Meteo para Córdoba, con tormenta (código 95).
const sampleResponse = `{"latitude":-31.318102,"longitude":-64.255066,"generationtime_ms":2.5,
"utc_offset_seconds":0,"timezone":"GMT","timezone_abbreviation":"GMT","elevation":466.0,
"current_units":{"time":"iso8601","interval":"seconds","temperature_2m":"°C","wind_speed_10m":"km/h",
"wind_gusts_10m":"km/h","wind_direction_10m":"°","visibility":"m","precipitation":"mm","weather_code":"wmo code"},
"current":{"time":"2026-10-19T19:15","interval":900,"temperature_2m":17.2,"wind_speed_10m":16.0,
"wind_gusts_10m":37.1,"wind_direction_10m":39,"visibility":50960.00,"precipitation":0.40,"weather_code":95}}`

func openMeteoServer(t *testing.T, body string) *OpenMeteo {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("latitude") != "-31.3236" || q.Get("wind_speed_unit") != "kmh" || q.Get("timezone") != "UTC" {
			t.Errorf("pedido inesperado: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	o := NewOpenMeteo()
	o.baseURL = srv.URL
	return o
}

func TestOpenMeteoLeeCadaCampo(t *testing.T) {
	obs, err := openMeteoServer(t, sampleResponse).Current(context.Background(), models.Cordoba)
	if err != nil {
		t.Fatal(err)
	}
	want := models.Conditions{
		TemperatureC:     17.2,
		WindSpeedKmh:     16.0,
		WindGustKmh:      37.1,
		WindDirectionDeg: 39,
		VisibilityM:      50960,
		PrecipitationMm:  0.4,
		Thunderstorm:     true,
	}
	if obs.Conditions != want {
		t.Errorf("condiciones = %+v, se esperaba %+v", obs.Conditions, want)
	}
	if !obs.ObservedAt.Equal(time.Date(2026, 10, 19, 19, 15, 0, 0, time.UTC)) || obs.Airport != "SACO" {
		t.Errorf("observación = %+v", obs)
	}
}

func TestOpenMeteoRechazaUnDatoFaltante(t *testing.T) {
	sinVisibilidad := strings.Replace(sampleResponse, `"visibility":50960.00,`, "", 1)
	_, err := openMeteoServer(t, sinVisibilidad).Current(context.Background(), models.Cordoba)
	if err == nil || !strings.Contains(err.Error(), "visibility") {
		t.Errorf("se esperaba un error por la visibilidad faltante, fue %v", err)
	}
}

func TestOpenMeteoRechazaUnDatoNulo(t *testing.T) {
	nulo := strings.Replace(sampleResponse, `"wind_gusts_10m":37.1`, `"wind_gusts_10m":null`, 1)
	_, err := openMeteoServer(t, nulo).Current(context.Background(), models.Cordoba)
	if err == nil || !strings.Contains(err.Error(), "wind_gusts_10m") {
		t.Errorf("se esperaba un error por las ráfagas nulas, fue %v", err)
	}
}
