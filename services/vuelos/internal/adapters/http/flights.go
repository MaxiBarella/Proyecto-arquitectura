package httpadapter

import (
	"fmt"
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

// pageSize es la cantidad de vuelos por página (14.4).
const pageSize = 20

func validStatus(s domain.Status) bool {
	switch s {
	case domain.PendingSlot, domain.Scheduled, domain.Delayed, domain.Completed, domain.Diverted, domain.Cancelled:
		return true
	}
	return false
}
