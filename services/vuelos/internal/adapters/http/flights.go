package httpadapter

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

const identityKey = "identity"

// Register suma las rutas de Vuelos (openapi.yaml). Todas las de /flights son
// solo para aerolíneas.
func Register(r gin.IRouter, cmds *commands.Service, qs *queries.Service) {
	h := &handlers{cmds: cmds, qs: qs}
	flights := r.Group("/flights", requireAirline())
	flights.POST("", h.publish)
	flights.GET("", h.listOwn)
	flights.GET("/:id", h.getOwn)
	flights.POST("/:id/cancel", h.cancel)
	flights.POST("/:id/slot-request", h.requestSlot)
}

type handlers struct {
	cmds *commands.Service
	qs   *queries.Service
}

// requireAirline lee la identidad de los encabezados que pone el gateway (15.6)
// y deja pasar solo a una aerolínea (CA-18, CA-19).
func requireAirline() gin.HandlerFunc {
	return func(c *gin.Context) {
		who := commands.Identity{
			UserID:  c.GetHeader("X-User-Id"),
			Role:    c.GetHeader("X-User-Role"),
			Airline: c.GetHeader("X-Airline"),
		}
		if who.UserID == "" || who.Role == "" {
			writeProblem(c, http.StatusUnauthorized, codeMissingIdentity, "El pedido no incluye X-User-Id o X-User-Role.", 0)
			return
		}
		if who.Role != commands.RoleAirline {
			writeProblem(c, http.StatusForbidden, codeForbiddenRole, "Solo una aerolínea puede operar vuelos.", 0)
			return
		}
		if who.Airline == "" {
			writeProblem(c, http.StatusUnauthorized, codeMissingIdentity, "El pedido no incluye X-Airline.", 0)
			return
		}
		c.Set(identityKey, who)
		c.Next()
	}
}

func identity(c *gin.Context) commands.Identity {
	return c.MustGet(identityKey).(commands.Identity)
}

type newFlightRequest struct {
	FlightNumber string    `json:"flight_number"`
	Type         string    `json:"type"`
	Scope        string    `json:"scope"`
	Origin       string    `json:"origin"`
	Destination  string    `json:"destination"`
	ScheduledAt  time.Time `json:"scheduled_at"`
}

type slotDTO struct {
	RunwayID   string    `json:"runway_id"`
	BlockStart time.Time `json:"block_start"`
}

type flightResponse struct {
	ID           string    `json:"id"`
	FlightNumber string    `json:"flight_number"`
	Airline      string    `json:"airline"`
	Type         string    `json:"type"`
	Scope        string    `json:"scope"`
	Origin       string    `json:"origin"`
	Destination  string    `json:"destination"`
	ScheduledAt  time.Time `json:"scheduled_at"`
	Status       string    `json:"status"`
	Slot         *slotDTO  `json:"slot"`
	DivertedTo   *string   `json:"diverted_to"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type flightPage struct {
	Items    []flightResponse `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}

func toResponse(f *domain.Flight) flightResponse {
	r := flightResponse{
		ID:           f.ID,
		FlightNumber: f.Number,
		Airline:      f.Airline,
		Type:         string(f.Type),
		Scope:        string(f.Scope),
		Origin:       f.Origin,
		Destination:  f.Destination,
		ScheduledAt:  f.ScheduledAt.UTC(),
		Status:       string(f.Status),
		CreatedAt:    f.CreatedAt.UTC(),
		UpdatedAt:    f.UpdatedAt.UTC(),
	}
	if f.Slot != nil {
		r.Slot = &slotDTO{RunwayID: f.Slot.RunwayID, BlockStart: f.Slot.BlockStart.UTC()}
	}
	if f.DivertedTo != "" {
		alternate := f.DivertedTo
		r.DivertedTo = &alternate
	}
	return r
}

// publish responde POST /flights (F-02).
func (h *handlers) publish(c *gin.Context) {
	var req newFlightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeProblem(c, http.StatusBadRequest, codeInvalidRequest, fmt.Sprintf("No se pudo leer el cuerpo: %v", err), 0)
		return
	}
	f, err := h.cmds.Publish(c.Request.Context(), identity(c), domain.NewFlightData{
		Number:      req.FlightNumber,
		Type:        domain.FlightType(req.Type),
		Scope:       domain.Scope(req.Scope),
		Origin:      req.Origin,
		Destination: req.Destination,
		ScheduledAt: req.ScheduledAt,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.Header("Location", "/flights/"+f.ID)
	c.JSON(http.StatusCreated, toResponse(f))
}

// listOwn responde GET /flights (F-07).
func (h *handlers) listOwn(c *gin.Context) {
	status := domain.Status(c.Query("status"))
	if status != "" && !validStatus(status) {
		writeProblem(c, http.StatusBadRequest, codeInvalidRequest, fmt.Sprintf("%q no es un estado válido.", status), 0)
		return
	}
	page := 1
	if raw := c.Query("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeProblem(c, http.StatusBadRequest, codeInvalidRequest, "page tiene que ser un número desde 1.", 0)
			return
		}
		page = n
	}
	flights, total, err := h.qs.ListOwn(c.Request.Context(), identity(c), status, page)
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]flightResponse, len(flights))
	for i := range flights {
		items[i] = toResponse(&flights[i])
	}
	c.JSON(http.StatusOK, flightPage{Items: items, Page: page, PageSize: pageSize, Total: total})
}

// getOwn responde GET /flights/:id.
func (h *handlers) getOwn(c *gin.Context) {
	f, err := h.qs.Own(c.Request.Context(), identity(c), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(f))
}

// cancel responde POST /flights/:id/cancel (F-06).
func (h *handlers) cancel(c *gin.Context) {
	f, err := h.cmds.Cancel(c.Request.Context(), identity(c), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(f))
}

type slotAccepted struct {
	FlightID string  `json:"flight_id"`
	Slot     slotDTO `json:"slot"`
	Status   string  `json:"status"`
}

// runwaysRetryAfter son los segundos que se le piden a la aerolínea antes de
// reintentar si Pistas no está disponible: lo que queda abierto el circuit
// breaker (13.2).
const runwaysRetryAfter = 30

// requestSlot responde POST /flights/:id/slot-request (F-03, F-05). Si Pistas
// asigna el slot responde 202: el vuelo pasa a programado con el evento (4.1).
func (h *handlers) requestSlot(c *gin.Context) {
	var req slotDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		writeProblem(c, http.StatusBadRequest, codeInvalidRequest, fmt.Sprintf("No se pudo leer el cuerpo: %v", err), 0)
		return
	}
	f, assigned, err := h.cmds.RequestSlot(c.Request.Context(), identity(c), c.Param("id"),
		domain.Slot{RunwayID: req.RunwayID, BlockStart: req.BlockStart})

	var taken *commands.SlotTakenError
	switch {
	case err == nil:
		c.JSON(http.StatusAccepted, slotAccepted{
			FlightID: f.ID,
			Slot:     slotDTO{RunwayID: assigned.RunwayID, BlockStart: assigned.BlockStart.UTC()},
			Status:   string(f.Status),
		})
	case errors.As(err, &taken):
		nearest := make([]slotDTO, len(taken.Nearest))
		for i, s := range taken.Nearest {
			nearest[i] = slotDTO{RunwayID: s.RunwayID, BlockStart: s.BlockStart.UTC()}
		}
		writeProblemBody(c, http.StatusConflict, problem{
			Code:             codeSlotTaken,
			Detail:           fmt.Sprintf("La pista %s ya tiene un vuelo en ese bloque.", req.RunwayID),
			NearestFreeSlots: nearest,
		}, 0)
	case errors.Is(err, commands.ErrRunwayClosed):
		writeProblem(c, http.StatusConflict, codeRunwayClosed, fmt.Sprintf("La pista %s está cerrada.", req.RunwayID), 0)
	case errors.Is(err, commands.ErrRunwayNotFound):
		writeProblem(c, http.StatusNotFound, codeRunwayNotFound, fmt.Sprintf("No existe la pista %s.", req.RunwayID), 0)
	case errors.Is(err, commands.ErrRunwaysUnavailable):
		slog.WarnContext(c.Request.Context(), "no se pudo pedir el slot", "error", err)
		writeProblem(c, http.StatusServiceUnavailable, codeRunwaysUnavailable,
			"No se pudo pedir el slot. El vuelo sigue como estaba.", runwaysRetryAfter)
	default:
		writeError(c, err)
	}
}

// pageSize es la cantidad de vuelos por página (14.4).
const pageSize = 20

func validStatus(s domain.Status) bool {
	switch s {
	case domain.PendingSlot, domain.Scheduled, domain.Delayed, domain.Completed, domain.Diverted, domain.Cancelled:
		return true
	}
	return false
}
