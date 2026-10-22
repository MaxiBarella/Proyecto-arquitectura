package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// problem es un error en formato Problem Details (RFC 9457), con el campo code propio (6.14).
type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}

// Códigos de error del contrato, más los de la ruta interna del forzado.
const (
	codeInvalidAirportCode       = "INVALID_AIRPORT_CODE"
	codeMissingAPIKey            = "MISSING_API_KEY"
	codeInvalidAPIKey            = "INVALID_API_KEY"
	codeAirportNotSupported      = "AIRPORT_NOT_SUPPORTED"
	codeRateLimitExceeded        = "RATE_LIMIT_EXCEEDED"
	codeWeatherSourceUnavailable = "WEATHER_SOURCE_UNAVAILABLE"
	codeForbidden                = "FORBIDDEN"
	codeInvalidBody              = "INVALID_BODY"
	codeInternalError            = "INTERNAL_ERROR"
)

var problemTypes = map[string]struct{ path, title string }{
	codeInvalidAirportCode:       {"/problems/invalid-airport-code", "Código de aeropuerto inválido"},
	codeMissingAPIKey:            {"/problems/missing-api-key", "Falta la clave"},
	codeInvalidAPIKey:            {"/problems/invalid-api-key", "Clave inválida"},
	codeAirportNotSupported:      {"/problems/airport-not-supported", "Aeropuerto no atendido"},
	codeRateLimitExceeded:        {"/problems/rate-limit-exceeded", "Límite de pedidos superado"},
	codeWeatherSourceUnavailable: {"/problems/weather-source-unavailable", "Fuente de clima no disponible"},
	codeForbidden:                {"/problems/forbidden", "Operación no permitida"},
	codeInvalidBody:              {"/problems/invalid-body", "Cuerpo del pedido inválido"},
	codeInternalError:            {"/problems/internal-error", "Error interno"},
}

// writeProblem corta el pedido y responde el error. Si retryAfter es mayor que
// cero, agrega el encabezado Retry-After con esos segundos.
func writeProblem(c *gin.Context, status int, code, detail string, retryAfter int) {
	t := problemTypes[code]
	if retryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
	}
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, problem{
		Type:     t.path,
		Title:    t.title,
		Status:   status,
		Detail:   detail,
		Instance: c.Request.URL.Path,
		Code:     code,
	})
}
