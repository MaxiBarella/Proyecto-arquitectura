package httpadapter

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

// boardRetryAfter son los segundos que se piden antes de reintentar si el índice no
// responde. Es el valor de ejemplo del contrato (BoardUnavailable), el mismo que se usa
// cuando Pistas no responde.
const boardRetryAfter = 30

// maxQueryLength es el largo máximo del texto a buscar. Un número de vuelo o una
// ciudad entran de sobra; más largo no encuentra nada y solo carga al índice.
const maxQueryLength = 60

// board responde GET /board (F-13, F-14): el tablero público, sin identidad (RN-30).
func (h *handlers) board(c *gin.Context) {
	f, problem := boardFilter(c)
	if problem != "" {
		writeProblem(c, http.StatusBadRequest, codeInvalidRequest, problem, 0)
		return
	}
	flights, total, err := h.qs.Board(c.Request.Context(), f, time.Now())
	if errors.Is(err, queries.ErrBoardUnavailable) {
		slog.WarnContext(c.Request.Context(), "no se pudo consultar el tablero", "error", err)
		writeProblem(c, http.StatusServiceUnavailable, codeBoardUnavailable, "El índice de búsqueda no responde.", boardRetryAfter)
		return
	}
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]flightResponse, len(flights))
	for i := range flights {
		items[i] = boardResponse(&flights[i])
	}
	c.JSON(http.StatusOK, flightPage{Items: items, Page: f.Page, PageSize: queries.BoardPageSize, Total: total})
}

// boardFilter lee los parámetros del tablero (RN-33 a RN-36). Devuelve el motivo si
// alguno no es válido.
func boardFilter(c *gin.Context) (queries.BoardFilter, string) {
	f := queries.BoardFilter{
		Type:    c.Query("type"),
		Status:  c.Query("status"),
		Airline: strings.ToUpper(strings.TrimSpace(c.Query("airline"))),
		Query:   strings.TrimSpace(c.Query("q")),
		Sort:    c.DefaultQuery("sort", queries.SortByTime),
		Page:    1,
	}
	if f.Type != "" && f.Type != string(domain.Arrival) && f.Type != string(domain.Departure) {
		return f, fmt.Sprintf("%q no es un tipo de operación válido: arrival o departure.", f.Type)
	}
	if f.Status != "" && !validStatus(domain.Status(f.Status)) {
		return f, fmt.Sprintf("%q no es un estado válido.", f.Status)
	}
	switch f.Sort {
	case queries.SortByTime, queries.SortByAirline, queries.SortByScope:
	default:
		return f, "sort tiene que ser scheduled_at, airline o scope."
	}
	if len([]rune(f.Query)) > maxQueryLength {
		return f, fmt.Sprintf("q puede tener hasta %d caracteres.", maxQueryLength)
	}
	if raw := c.Query("date"); raw != "" {
		d, err := queries.ParseBoardDate(raw)
		if err != nil {
			return f, "date tiene que ser una fecha con la forma AAAA-MM-DD."
		}
		f.Date = d
	}
	if raw := c.Query("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return f, "page tiene que ser un número desde 1."
		}
		f.Page = n
	}
	return f, ""
}

// boardResponse arma un vuelo del tablero con la forma de Flight del contrato.
func boardResponse(f *queries.IndexedFlight) flightResponse {
	r := flightResponse{
		ID:           f.ID,
		FlightNumber: f.FlightNumber,
		Airline:      f.Airline,
		Type:         f.Type,
		Scope:        f.Scope,
		Origin:       f.Origin,
		Destination:  f.Destination,
		ScheduledAt:  f.ScheduledAt.UTC(),
		Status:       f.Status,
		DivertedTo:   f.DivertedTo,
		CreatedAt:    f.CreatedAt.UTC(),
		UpdatedAt:    f.UpdatedAt.UTC(),
	}
	if f.Slot != nil {
		r.Slot = &slotDTO{RunwayID: f.Slot.RunwayID, BlockStart: f.Slot.BlockStart.UTC()}
	}
	return r
}
