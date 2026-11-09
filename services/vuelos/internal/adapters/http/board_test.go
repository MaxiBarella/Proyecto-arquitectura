package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

type fakeBoard struct {
	got     queries.BoardSearch
	flights []queries.IndexedFlight
	err     error
}

func (f *fakeBoard) Search(_ context.Context, s queries.BoardSearch) ([]queries.IndexedFlight, int64, error) {
	f.got = s
	return f.flights, int64(len(f.flights)), f.err
}

func newBoardServer(board *fakeBoard) *testServer {
	gin.SetMode(gin.TestMode)
	store := newMemStore()
	r := gin.New()
	Register(r, commands.New(store, &fakeSlots{}), queries.New(store).WithBoard(board))
	return &testServer{router: r, store: store}
}

// El tablero es público: responde sin los encabezados de identidad (RN-30).
func TestBoardIsPublic(t *testing.T) {
	board := &fakeBoard{flights: []queries.IndexedFlight{{FlightUpdatedData: commands.FlightUpdatedData{
		ID: "v1", FlightNumber: "AR1550", Airline: "AR", Type: "departure", Scope: "domestic", Status: "pending_slot"}}}}
	s := newBoardServer(board)
	rec := s.do(t, http.MethodGet, "/board?airline=ar&q=%20cordoba%20&type=departure&sort=airline&page=2&date=2026-10-23", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	page := decode[flightPage](t, rec)
	if page.Page != 2 || page.PageSize != 20 || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Slot != nil {
		t.Errorf("página: %+v", page)
	}
	got := board.got
	if got.Airline != "AR" || got.Query != "cordoba" || got.Type != "departure" || got.Sort != "airline" || got.To.IsZero() {
		t.Errorf("filtro: %+v", got)
	}
}

func TestBoardRejectsInvalidParams(t *testing.T) {
	s := newBoardServer(&fakeBoard{})
	for _, q := range []string{"type=otro", "status=volando", "sort=precio", "date=23/10/2026", "page=0", "page=x",
		"q=" + strings.Repeat("a", 61)} {
		rec := s.do(t, http.MethodGet, "/board?"+q, nil, nil)
		if rec.Code != http.StatusBadRequest || decode[problem](t, rec).Code != codeInvalidRequest {
			t.Errorf("%s: got %d %s", q, rec.Code, rec.Body.String())
		}
	}
}

// Si Solr no responde, 503 con Retry-After (contrato, BoardUnavailable).
func TestBoardUnavailable(t *testing.T) {
	s := newBoardServer(&fakeBoard{err: errors.New("solr caído")})
	rec := s.do(t, http.MethodGet, "/board", nil, nil)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "30" ||
		decode[problem](t, rec).Code != codeBoardUnavailable {
		t.Errorf("got %d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
}
