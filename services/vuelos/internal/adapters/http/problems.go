package httpadapter

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// problem es un error en formato Problem Details (RFC 9457), con el campo code
// propio, igual que en Clima (6.14).
type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
	// NearestFreeSlots va solo en SLOT_TAKEN (RN-09).
	NearestFreeSlots []slotDTO `json:"nearest_free_slots,omitempty"`
}

// Códigos de error del contrato (openapi.yaml).
const (
	codeInvalidRequest     = "INVALID_REQUEST"
	codeMissingIdentity    = "MISSING_IDENTITY"
	codeForbiddenRole      = "FORBIDDEN_ROLE"
	codeNotFlightOwner     = "NOT_FLIGHT_OWNER"
	codeFlightNotFound     = "FLIGHT_NOT_FOUND"
	codeRunwayNotFound     = "RUNWAY_NOT_FOUND"
	codeSlotTaken          = "SLOT_TAKEN"
	codeRunwayClosed       = "RUNWAY_CLOSED"
	codeInvalidTransition  = "INVALID_TRANSITION"
	codeRunwaysUnavailable = "RUNWAYS_UNAVAILABLE"
	codeInternalError      = "INTERNAL_ERROR"
)

var problemTypes = map[string]struct{ path, title string }{
	codeInvalidRequest:     {"/problems/invalid-request", "Pedido inválido"},
	codeMissingIdentity:    {"/problems/missing-identity", "Falta la identidad"},
	codeForbiddenRole:      {"/problems/forbidden-role", "Operación no permitida para el rol"},
	codeNotFlightOwner:     {"/problems/not-flight-owner", "El vuelo es de otra aerolínea"},
	codeFlightNotFound:     {"/problems/flight-not-found", "Vuelo no encontrado"},
	codeRunwayNotFound:     {"/problems/runway-not-found", "Pista no encontrada"},
	codeSlotTaken:          {"/problems/slot-taken", "Slot ocupado"},
	codeRunwayClosed:       {"/problems/runway-closed", "Pista cerrada"},
	codeInvalidTransition:  {"/problems/invalid-transition", "Transición no permitida"},
	codeRunwaysUnavailable: {"/problems/runways-unavailable", "Pistas no disponible"},
	codeInternalError:      {"/problems/internal-error", "Error interno"},
}

// writeProblem corta el pedido y responde el error. Si retryAfter es mayor que
// cero, agrega el encabezado Retry-After con esos segundos.
func writeProblem(c *gin.Context, status int, code, detail string, retryAfter int) {
	writeProblemBody(c, status, problem{Code: code, Detail: detail}, retryAfter)
}

func writeProblemBody(c *gin.Context, status int, p problem, retryAfter int) {
	t := problemTypes[p.Code]
	p.Type, p.Title, p.Status, p.Instance = t.path, t.title, status, c.Request.URL.Path
	if retryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
	}
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, p)
}

// writeError traduce un error del dominio a su respuesta. Lo que no reconoce es
// un 500 y queda en el log.
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidFlight):
		writeProblem(c, http.StatusBadRequest, codeInvalidRequest, err.Error(), 0)
	case errors.Is(err, domain.ErrNotOwner):
		writeProblem(c, http.StatusForbidden, codeNotFlightOwner, err.Error(), 0)
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(c, http.StatusNotFound, codeFlightNotFound, err.Error(), 0)
	case errors.Is(err, domain.ErrInvalidTransition):
		writeProblem(c, http.StatusConflict, codeInvalidTransition, err.Error(), 0)
	default:
		slog.ErrorContext(c.Request.Context(), "error al atender el pedido", "error", err)
		writeProblem(c, http.StatusInternalServerError, codeInternalError, "", 0)
	}
}
