package services

import (
	"reflect"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

// calm son condiciones que no rompen ningún umbral.
var calm = models.Conditions{
	TemperatureC:     20,
	WindSpeedKmh:     10,
	WindGustKmh:      20,
	WindDirectionDeg: 180,
	VisibilityM:      10000,
}

func TestEvaluateAptitude(t *testing.T) {
	tests := []struct {
		name    string
		change  func(c *models.Conditions)
		reasons []string
	}{
		{"condiciones tranquilas", func(c *models.Conditions) {}, []string{}},
		{"viento justo en el umbral", func(c *models.Conditions) { c.WindSpeedKmh = 50 }, []string{}},
		{"viento sobre el umbral", func(c *models.Conditions) { c.WindSpeedKmh = 50.1 }, []string{models.ReasonHighWind}},
		{"ráfagas justo en el umbral", func(c *models.Conditions) { c.WindGustKmh = 65 }, []string{}},
		{"ráfagas sobre el umbral", func(c *models.Conditions) { c.WindGustKmh = 65.1 }, []string{models.ReasonHighGusts}},
		{"visibilidad justo en el umbral", func(c *models.Conditions) { c.VisibilityM = 800 }, []string{}},
		{"visibilidad bajo el umbral", func(c *models.Conditions) { c.VisibilityM = 799 }, []string{models.ReasonLowVisibility}},
		{"tormenta", func(c *models.Conditions) { c.Thunderstorm = true }, []string{models.ReasonThunderstorm}},
		{
			"varios motivos, en orden fijo",
			func(c *models.Conditions) {
				c.Thunderstorm = true
				c.VisibilityM = 300
				c.WindGustKmh = 90
				c.WindSpeedKmh = 70
			},
			[]string{models.ReasonHighWind, models.ReasonHighGusts, models.ReasonLowVisibility, models.ReasonThunderstorm},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := calm
			tt.change(&c)
			got := EvaluateAptitude(c)
			if !reflect.DeepEqual(got.Reasons, tt.reasons) {
				t.Errorf("motivos = %v, se esperaba %v", got.Reasons, tt.reasons)
			}
			if got.Operable != (len(tt.reasons) == 0) {
				t.Errorf("operable = %v con motivos %v", got.Operable, got.Reasons)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		age  time.Duration
		want bool
	}{
		{"recién observado", 0, false},
		{"de 15 minutos", 15 * time.Minute, false},
		{"de exactamente 30 minutos", 30 * time.Minute, false},
		{"de 30 minutos y un segundo", 30*time.Minute + time.Second, true},
		{"de una hora", time.Hour, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStale(now.Add(-tt.age), now); got != tt.want {
				t.Errorf("IsStale con %v = %v, se esperaba %v", tt.age, got, tt.want)
			}
		})
	}
}
