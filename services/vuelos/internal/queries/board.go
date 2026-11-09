package queries

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// BoardPageSize es la cantidad de vuelos por página del tablero (14.4, RN-36).
const BoardPageSize = 20

// Órdenes del tablero (RN-35). Los empates se ordenan por horario y después por número
// de vuelo, así la paginación no repite ni saltea vuelos (decisión 18, pendiente 21).
const (
	SortByTime    = "scheduled_at"
	SortByAirline = "airline"
	SortByScope   = "scope"
)

// ErrBoardUnavailable es un tablero que no se pudo consultar porque el índice no
// responde. La capa HTTP lo traduce a 503 BOARD_UNAVAILABLE.
var ErrBoardUnavailable = errors.New("el índice del tablero no responde")

// argentina es la hora de Argentina (UTC−3), sin horario de verano. La fecha del
// tablero es el día en esa hora (14.9, RN-33).
var argentina = time.FixedZone("ART", -3*60*60)

// BoardFilter es lo que pide el visitante del tablero (RN-33 a RN-36).
type BoardFilter struct {
	Type    string
	Status  string
	Airline string
	// Date es el día pedido, en hora de Argentina. Si es cero, se muestran los vuelos
	// de hoy en adelante (RN-33).
	Date  time.Time
	Query string
	Sort  string
	Page  int
}

// BoardSearch es la consulta que se le hace al índice: el filtro, con el rango de
// horarios ya calculado.
type BoardSearch struct {
	BoardFilter
	// From y To acotan el horario del tablero (RN-39): From incluido, To excluido.
	// To en cero quiere decir sin límite.
	From, To time.Time
}

// BoardIndex busca en el índice del tablero. Devuelve una página y el total.
type BoardIndex interface {
	Search(ctx context.Context, s BoardSearch) ([]IndexedFlight, int64, error)
}

// WithBoard le da al servicio el índice del tablero.
func (s *Service) WithBoard(index BoardIndex) *Service {
	s.board = index
	return s
}

// Board devuelve una página del tablero público (F-13, F-14) y cuántos vuelos cumplen
// el filtro. now es el momento de la consulta, para "de hoy en adelante".
func (s *Service) Board(ctx context.Context, f BoardFilter, now time.Time) ([]IndexedFlight, int64, error) {
	if s.board == nil {
		return nil, 0, ErrBoardUnavailable
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Sort == "" {
		f.Sort = SortByTime
	}
	search := BoardSearch{BoardFilter: f}
	if f.Date.IsZero() {
		// Sin fecha, de hoy en adelante: desde las 00:00 de hoy en hora de Argentina.
		search.From = startOfDay(now)
	} else {
		search.From = startOfDay(f.Date)
		search.To = search.From.AddDate(0, 0, 1)
	}
	flights, total, err := s.board.Search(ctx, search)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrBoardUnavailable, err)
	}
	return flights, total, nil
}

// startOfDay es las 00:00 en hora de Argentina del día de t, en UTC.
func startOfDay(t time.Time) time.Time {
	local := t.In(argentina)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, argentina).UTC()
}

// ParseBoardDate lee una fecha del tablero (AAAA-MM-DD) como un día en hora de Argentina.
func ParseBoardDate(raw string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", raw, argentina)
}
