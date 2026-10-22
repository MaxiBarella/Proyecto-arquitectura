package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

const openMeteoURL = "https://api.open-meteo.com/v1/forecast"

// openMeteoTimeout es cuánto se espera a Open-Meteo. Si no responde, se vuelve a
// intentar en la próxima consulta periódica.
const openMeteoTimeout = 10 * time.Second

// Códigos WMO de tormenta eléctrica, con o sin granizo.
var thunderstormCodes = map[int]bool{95: true, 96: true, 99: true}

// OpenMeteo consulta el clima actual a Open-Meteo (6.8).
type OpenMeteo struct {
	client  *http.Client
	baseURL string
}

func NewOpenMeteo() *OpenMeteo {
	return &OpenMeteo{client: &http.Client{Timeout: openMeteoTimeout}, baseURL: openMeteoURL}
}

// Los valores son punteros para distinguir un dato que no vino de un 0: si falta
// alguno, la observación se descarta en vez de leerse como 0 (por ejemplo, 0 metros
// de visibilidad cerraría las pistas).
type openMeteoResponse struct {
	Current struct {
		Time             string   `json:"time"`
		TemperatureC     *float64 `json:"temperature_2m"`
		WindSpeedKmh     *float64 `json:"wind_speed_10m"`
		WindGustKmh      *float64 `json:"wind_gusts_10m"`
		WindDirectionDeg *float64 `json:"wind_direction_10m"`
		VisibilityM      *float64 `json:"visibility"`
		PrecipitationMm  *float64 `json:"precipitation"`
		WeatherCode      *int     `json:"weather_code"`
	} `json:"current"`
}

// Current devuelve la observación actual del aeropuerto.
func (o *OpenMeteo) Current(ctx context.Context, airport models.Airport) (models.Observation, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(airport.Latitude, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(airport.Longitude, 'f', 4, 64))
	q.Set("current", "temperature_2m,wind_speed_10m,wind_gusts_10m,wind_direction_10m,visibility,precipitation,weather_code")
	q.Set("wind_speed_unit", "kmh")
	q.Set("timezone", "UTC")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return models.Observation{}, err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return models.Observation{}, fmt.Errorf("consultar Open-Meteo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Open-Meteo explica el motivo en el campo "reason", por ejemplo si se pasó
		// un límite por minuto, por hora o por día.
		var failure struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&failure)
		if failure.Reason != "" {
			return models.Observation{}, fmt.Errorf("Open-Meteo respondió %d: %s", resp.StatusCode, failure.Reason)
		}
		return models.Observation{}, fmt.Errorf("Open-Meteo respondió %d", resp.StatusCode)
	}

	var body openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return models.Observation{}, fmt.Errorf("leer la respuesta de Open-Meteo: %w", err)
	}
	// Open-Meteo devuelve la hora sin zona; con timezone=UTC, es UTC.
	observedAt, err := time.Parse("2006-01-02T15:04", body.Current.Time)
	if err != nil {
		return models.Observation{}, fmt.Errorf("fecha inválida de Open-Meteo %q: %w", body.Current.Time, err)
	}

	c := body.Current
	missing := missingFields(map[string]bool{
		"temperature_2m":     c.TemperatureC == nil,
		"wind_speed_10m":     c.WindSpeedKmh == nil,
		"wind_gusts_10m":     c.WindGustKmh == nil,
		"wind_direction_10m": c.WindDirectionDeg == nil,
		"visibility":         c.VisibilityM == nil,
		"precipitation":      c.PrecipitationMm == nil,
		"weather_code":       c.WeatherCode == nil,
	})
	if len(missing) > 0 {
		return models.Observation{}, fmt.Errorf("Open-Meteo no mandó %s", strings.Join(missing, ", "))
	}
	return models.Observation{
		Airport:    airport.ICAO,
		ObservedAt: observedAt,
		Conditions: models.Conditions{
			TemperatureC:     *c.TemperatureC,
			WindSpeedKmh:     *c.WindSpeedKmh,
			WindGustKmh:      *c.WindGustKmh,
			WindDirectionDeg: int(*c.WindDirectionDeg),
			VisibilityM:      *c.VisibilityM,
			PrecipitationMm:  *c.PrecipitationMm,
			Thunderstorm:     thunderstormCodes[*c.WeatherCode],
		},
	}, nil
}

// missingFields devuelve, en orden alfabético, los campos marcados como faltantes.
func missingFields(fields map[string]bool) []string {
	var missing []string
	for name, isMissing := range fields {
		if isMissing {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}
