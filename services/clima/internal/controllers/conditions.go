package controllers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/services"
)

// noDataRetryAfter son los segundos que se le piden al consumidor antes de
// reintentar cuando no hay un dato reciente (9.17).
const noDataRetryAfter = 60

var icaoPattern = regexp.MustCompile(`^[A-Z]{4}$`)

// WeatherReader es lo que necesitan los controllers del servicio de clima.
type WeatherReader interface {
	Airport() models.Airport
	Current(ctx context.Context) (models.Observation, models.Operability, error)
	SetForced(ctx context.Context, in services.ForcedInput) (models.Conditions, models.Operability, error)
	ClearForced(ctx context.Context) error
}

// ConditionsController atiende la API pública de Clima.
type ConditionsController struct {
	weather WeatherReader
}

func NewConditionsController(weather WeatherReader) *ConditionsController {
	return &ConditionsController{weather: weather}
}

type airportResponse struct {
	ICAO string `json:"icao"`
	Name string `json:"name"`
}

type conditionsResponse struct {
	Airport     airportResponse    `json:"airport"`
	ObservedAt  time.Time          `json:"observed_at"`
	Conditions  models.Conditions  `json:"conditions"`
	Operability models.Operability `json:"operability"`
}

// Get responde GET /airports/:icao/conditions.
func (ctl *ConditionsController) Get(c *gin.Context) {
	if !ctl.checkAirport(c) {
		return
	}
	obs, apt, err := ctl.weather.Current(c.Request.Context())
	if errors.Is(err, services.ErrNoData) {
		writeProblem(c, http.StatusServiceUnavailable, codeWeatherSourceUnavailable,
			"El último dato de clima tiene más de 30 minutos y no se pudo renovar.", noDataRetryAfter)
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo leer el clima", "error", err)
		writeProblem(c, http.StatusInternalServerError, codeInternalError, "", 0)
		return
	}
	airport := ctl.weather.Airport()
	c.JSON(http.StatusOK, conditionsResponse{
		Airport:     airportResponse{ICAO: airport.ICAO, Name: airport.Name},
		ObservedAt:  obs.ObservedAt.UTC(),
		Conditions:  obs.Conditions,
		Operability: apt,
	})
}

type forcedResponse struct {
	Airport     airportResponse    `json:"airport"`
	Conditions  models.Conditions  `json:"conditions"`
	Operability models.Operability `json:"operability"`
}

// PutForced responde PUT /internal/airports/:icao/forced-conditions (9.13).
func (ctl *ConditionsController) PutForced(c *gin.Context) {
	if !ctl.checkAirport(c) {
		return
	}
	var in services.ForcedInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeProblem(c, http.StatusBadRequest, codeInvalidBody, fmt.Sprintf("No se pudo leer el cuerpo: %v", err), 0)
		return
	}
	conditions, apt, err := ctl.weather.SetForced(c.Request.Context(), in)
	if errors.Is(err, services.ErrInvalidForced) {
		writeProblem(c, http.StatusBadRequest, codeInvalidBody, err.Error(), 0)
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo forzar la condición", "error", err)
		writeProblem(c, http.StatusInternalServerError, codeInternalError, "", 0)
		return
	}
	airport := ctl.weather.Airport()
	c.JSON(http.StatusOK, forcedResponse{
		Airport:     airportResponse{ICAO: airport.ICAO, Name: airport.Name},
		Conditions:  conditions,
		Operability: apt,
	})
}

// DeleteForced responde DELETE /internal/airports/:icao/forced-conditions (9.13).
func (ctl *ConditionsController) DeleteForced(c *gin.Context) {
	if !ctl.checkAirport(c) {
		return
	}
	if err := ctl.weather.ClearForced(c.Request.Context()); err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo quitar la condición forzada", "error", err)
		writeProblem(c, http.StatusInternalServerError, codeInternalError, "", 0)
		return
	}
	c.Status(http.StatusNoContent)
}

// checkAirport valida el código de la ruta: 400 si no es ICAO, 404 si no se atiende.
func (ctl *ConditionsController) checkAirport(c *gin.Context) bool {
	icao := c.Param("icao")
	if !icaoPattern.MatchString(icao) {
		writeProblem(c, http.StatusBadRequest, codeInvalidAirportCode,
			fmt.Sprintf("'%s' no es un código ICAO: se esperan cuatro letras mayúsculas.", icao), 0)
		return false
	}
	if icao != ctl.weather.Airport().ICAO {
		writeProblem(c, http.StatusNotFound, codeAirportNotSupported,
			fmt.Sprintf("%s no es un aeropuerto atendido. Hoy solo se atiende %s.", icao, ctl.weather.Airport().ICAO), 0)
		return false
	}
	return true
}
