package services

import (
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

// Umbrales de aptitud (6.11). Alcanza con que se cumpla uno para que no se pueda operar.
const (
	maxWindSpeedKmh = 50.0
	maxWindGustKmh  = 65.0
	minVisibilityM  = 800.0
)

// MaxDataAge es la antigüedad a partir de la cual el último dato se considera viejo (9.2).
const MaxDataAge = 30 * time.Minute

// EvaluateAptitude calcula si se puede operar con estas condiciones. Los motivos
// salen siempre en el mismo orden, para que dos evaluaciones iguales se puedan comparar.
func EvaluateAptitude(c models.Conditions) models.Operability {
	reasons := []string{}
	if c.WindSpeedKmh > maxWindSpeedKmh {
		reasons = append(reasons, models.ReasonHighWind)
	}
	if c.WindGustKmh > maxWindGustKmh {
		reasons = append(reasons, models.ReasonHighGusts)
	}
	if c.VisibilityM < minVisibilityM {
		reasons = append(reasons, models.ReasonLowVisibility)
	}
	if c.Thunderstorm {
		reasons = append(reasons, models.ReasonThunderstorm)
	}
	return models.Operability{Operable: len(reasons) == 0, Reasons: reasons}
}

// IsStale dice si un dato observado en observedAt ya es viejo en el momento now.
func IsStale(observedAt, now time.Time) bool {
	return now.Sub(observedAt) > MaxDataAge
}
