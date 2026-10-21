package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

// memStore es una base de vuelos en memoria que se comporta como la de MySQL: si
// un cambio falla, no guarda ni el vuelo ni sus eventos.
type memStore struct {
	mu      sync.Mutex
	flights map[string]domain.Flight
	outbox  []domain.Event
}

func newMemStore() *memStore { return &memStore{flights: map[string]domain.Flight{}} }

func (m *memStore) Create(_ context.Context, f *domain.Flight, events ...domain.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flights[f.ID] = *f
	m.outbox = append(m.outbox, events...)
	return nil
}

func (m *memStore) Get(_ context.Context, id string) (*domain.Flight, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.flights[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &f, nil
}

func (m *memStore) Update(_ context.Context, id string, change domain.Change) (*domain.Flight, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.flights[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	events, err := change(&f)
	if err != nil {
		return nil, err
	}
	f.Version++
	for i := range events {
		events[i].DataVersion = f.Version
	}
	m.flights[id] = f
	m.outbox = append(m.outbox, events...)
	return &f, nil
}

func (m *memStore) ListByAirline(_ context.Context, airline string, status domain.Status, page int) ([]domain.Flight, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var all []domain.Flight
	for _, f := range m.flights {
		if f.Airline == airline && (status == "" || f.Status == status) {
			all = append(all, f)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].EffectiveTime().Before(all[j].EffectiveTime()) })
	start := min((page-1)*pageSize, len(all))
	end := min(start+pageSize, len(all))
	return all[start:end], int64(len(all)), nil
}

// fakeSlots hace de Pistas: responde lo que el test le indique y guarda lo que le piden.
type fakeSlots struct {
	err error
	got []commands.SlotRequest
}

func (f *fakeSlots) RequestSlot(_ context.Context, req commands.SlotRequest) (domain.Slot, error) {
	f.got = append(f.got, req)
	if f.err != nil {
		return domain.Slot{}, f.err
	}
	return req.Slot, nil
}

type testServer struct {
	router *gin.Engine
	store  *memStore
	slots  *fakeSlots
}

func newServer() *testServer {
	gin.SetMode(gin.TestMode)
	store := newMemStore()
	slots := &fakeSlots{}
	r := gin.New()
	Register(r, commands.New(store, slots), queries.New(store))
	return &testServer{router: r, store: store, slots: slots}
}

// Encabezados de identidad que pone el gateway (15.6).
var (
	aerolineasArgentinas = map[string]string{"X-User-Id": "u1", "X-User-Role": "airline", "X-Airline": "AR"}
	latam                = map[string]string{"X-User-Id": "u2", "X-User-Role": "airline", "X-Airline": "LA"}
	operador             = map[string]string{"X-User-Id": "u3", "X-User-Role": "operator"}
)

func (s *testServer) do(t *testing.T, method, path string, headers map[string]string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("respuesta inválida %q: %v", rec.Body.String(), err)
	}
	return v
}

func departure() map[string]any {
	return map[string]any{
		"flight_number": "AR1550",
		"type":          "departure",
		"scope":         "domestic",
		"origin":        "Córdoba",
		"destination":   "Buenos Aires",
		"scheduled_at":  time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute).Format(time.RFC3339),
	}
}

func (s *testServer) publish(t *testing.T, headers map[string]string) flightResponse {
	t.Helper()
	rec := s.do(t, http.MethodPost, "/flights", headers, departure())
	if rec.Code != http.StatusCreated {
		t.Fatalf("publicar: %d %s", rec.Code, rec.Body.String())
	}
	return decode[flightResponse](t, rec)
}

func TestPublish(t *testing.T) {
	s := newServer()
	rec := s.do(t, http.MethodPost, "/flights", aerolineasArgentinas, departure())
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	f := decode[flightResponse](t, rec)
	if f.Status != "pending_slot" || f.Airline != "AR" || f.Slot != nil || f.DivertedTo != nil {
		t.Errorf("tiene que quedar pendiente, sin slot y de AR (CA-01): %+v", f)
	}
	if rec.Header().Get("Location") != "/flights/"+f.ID {
		t.Errorf("Location = %q", rec.Header().Get("Location"))
	}
}

func TestPublishRejectsInvalidData(t *testing.T) {
	s := newServer()
	cases := map[string]func(map[string]any){
		"origen igual al destino (CA-34)": func(b map[string]any) { b["destination"] = "Cordoba" },
		"horario en el pasado (CA-35)":    func(b map[string]any) { b["scheduled_at"] = "2020-01-01T10:00:00Z" },
		"número de vuelo inválido":        func(b map[string]any) { b["flight_number"] = "??" },
		"fecha con formato inválido":      func(b map[string]any) { b["scheduled_at"] = "mañana" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			body := departure()
			change(body)
			rec := s.do(t, http.MethodPost, "/flights", aerolineasArgentinas, body)
			if rec.Code != http.StatusBadRequest || decode[problem](t, rec).Code != codeInvalidRequest {
				t.Errorf("got %d %s", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Content-Type = %q", ct)
			}
		})
	}
}

func TestIdentity(t *testing.T) {
	s := newServer()
	cases := []struct {
		name    string
		headers map[string]string
		status  int
		code    string
	}{
		{"sin identidad", nil, http.StatusUnauthorized, codeMissingIdentity},
		{"aerolínea sin X-Airline", map[string]string{"X-User-Id": "u1", "X-User-Role": "airline"}, http.StatusUnauthorized, codeMissingIdentity},
		{"operador (CA-19 al revés)", operador, http.StatusForbidden, codeForbiddenRole},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := s.do(t, http.MethodPost, "/flights", c.headers, departure())
			if rec.Code != c.status || decode[problem](t, rec).Code != c.code {
				t.Errorf("got %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestEachAirlineSeesOnlyItsFlights(t *testing.T) {
	s := newServer()
	ar := s.publish(t, aerolineasArgentinas)
	s.publish(t, latam)

	rec := s.do(t, http.MethodGet, "/flights", aerolineasArgentinas, nil)
	page := decode[flightPage](t, rec)
	if rec.Code != http.StatusOK || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != ar.ID || page.PageSize != 20 {
		t.Errorf("AR tiene que ver solo su vuelo (RN-30): %d %+v", rec.Code, page)
	}

	if rec := s.do(t, http.MethodGet, "/flights/"+ar.ID, aerolineasArgentinas, nil); rec.Code != http.StatusOK {
		t.Errorf("su propio vuelo: %d", rec.Code)
	}
	rec = s.do(t, http.MethodGet, "/flights/"+ar.ID, latam, nil)
	if rec.Code != http.StatusForbidden || decode[problem](t, rec).Code != codeNotFlightOwner {
		t.Errorf("el vuelo de otra aerolínea: %d %s", rec.Code, rec.Body.String())
	}
	rec = s.do(t, http.MethodGet, "/flights/"+domain.NewID(), aerolineasArgentinas, nil)
	if rec.Code != http.StatusNotFound || decode[problem](t, rec).Code != codeFlightNotFound {
		t.Errorf("vuelo inexistente: %d %s", rec.Code, rec.Body.String())
	}
}

func TestListValidatesQuery(t *testing.T) {
	s := newServer()
	for _, q := range []string{"?status=volando", "?page=0", "?page=dos"} {
		if rec := s.do(t, http.MethodGet, "/flights"+q, aerolineasArgentinas, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d", q, rec.Code)
		}
	}
	if rec := s.do(t, http.MethodGet, "/flights?status=pending_slot&page=2", aerolineasArgentinas, nil); rec.Code != http.StatusOK {
		t.Errorf("filtro válido: got %d", rec.Code)
	}
}

func TestCancel(t *testing.T) {
	s := newServer()
	f := s.publish(t, aerolineasArgentinas)

	// Otra aerolínea no lo cancela (CA-26).
	rec := s.do(t, http.MethodPost, "/flights/"+f.ID+"/cancel", latam, nil)
	if rec.Code != http.StatusForbidden || decode[problem](t, rec).Code != codeNotFlightOwner {
		t.Fatalf("otra aerolínea: %d %s", rec.Code, rec.Body.String())
	}

	rec = s.do(t, http.MethodPost, "/flights/"+f.ID+"/cancel", aerolineasArgentinas, nil)
	if rec.Code != http.StatusOK || decode[flightResponse](t, rec).Status != "cancelled" {
		t.Fatalf("la dueña: %d %s", rec.Code, rec.Body.String())
	}
	// Deja "vuelo cancelado" en el outbox, con los datos de la 4.12.
	if len(s.store.outbox) != 1 {
		t.Fatalf("eventos en el outbox: %d, want 1", len(s.store.outbox))
	}
	e := s.store.outbox[0]
	if e.Type != "vuelos.vuelo.cancelado" || e.DataVersion != 2 || e.ID == "" || e.CorrelationID == "" {
		t.Errorf("evento: %+v", e)
	}
	if data, _ := json.Marshal(e.Data); string(data) != `{"flight_id":"`+f.ID+`"}` {
		t.Errorf("datos del evento: %s", data)
	}

	// Ya cancelado, no se vuelve a cancelar (CA-15).
	rec = s.do(t, http.MethodPost, "/flights/"+f.ID+"/cancel", aerolineasArgentinas, nil)
	if rec.Code != http.StatusConflict || decode[problem](t, rec).Code != codeInvalidTransition {
		t.Errorf("dos veces: %d %s", rec.Code, rec.Body.String())
	}
	if len(s.store.outbox) != 1 {
		t.Errorf("el rechazo no tiene que dejar eventos: %d", len(s.store.outbox))
	}
}

func slotBody(runway string, start time.Time) map[string]any {
	return map[string]any{"runway_id": runway, "block_start": start.Format(time.RFC3339)}
}

func TestRequestSlotAccepted(t *testing.T) {
	s := newServer()
	f := s.publish(t, aerolineasArgentinas)
	start := f.ScheduledAt.Truncate(15 * time.Minute)

	rec := s.do(t, http.MethodPost, "/flights/"+f.ID+"/slot-request", aerolineasArgentinas, slotBody("05-23", start))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	got := decode[slotAccepted](t, rec)
	// El vuelo sigue pendiente: pasa a programado con el evento (4.1).
	if got.Status != "pending_slot" || got.Slot.RunwayID != "05-23" || !got.Slot.BlockStart.Equal(start) {
		t.Errorf("respuesta: %+v", got)
	}
	// A Pistas le llega el tipo del vuelo, para la prioridad de los arribos (4.12).
	if len(s.slots.got) != 1 || s.slots.got[0].FlightType != domain.Departure || s.slots.got[0].FlightID != f.ID {
		t.Errorf("pedido a Pistas: %+v", s.slots.got)
	}
}

func TestRequestSlotErrors(t *testing.T) {
	nearest := []domain.Slot{{RunwayID: "18-36", BlockStart: time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)}}
	cases := []struct {
		name       string
		pistas     error
		status     int
		code       string
		retryAfter string
	}{
		{"slot ocupado (CA-03)", &commands.SlotTakenError{Nearest: nearest}, http.StatusConflict, codeSlotTaken, ""},
		{"pista cerrada (CA-23)", commands.ErrRunwayClosed, http.StatusConflict, codeRunwayClosed, ""},
		{"pista inexistente", commands.ErrRunwayNotFound, http.StatusNotFound, codeRunwayNotFound, ""},
		{"Pistas no responde (13.1)", commands.ErrRunwaysUnavailable, http.StatusServiceUnavailable, codeRunwaysUnavailable, "30"},
		{"el vuelo ya tiene slot en Pistas", domain.ErrInvalidTransition, http.StatusConflict, codeInvalidTransition, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newServer()
			f := s.publish(t, aerolineasArgentinas)
			s.slots.err = c.pistas
			rec := s.do(t, http.MethodPost, "/flights/"+f.ID+"/slot-request", aerolineasArgentinas,
				slotBody("05-23", f.ScheduledAt.Truncate(15*time.Minute)))
			p := decode[problem](t, rec)
			if rec.Code != c.status || p.Code != c.code || rec.Header().Get("Retry-After") != c.retryAfter {
				t.Errorf("got %d %s, Retry-After %q", rec.Code, rec.Body.String(), rec.Header().Get("Retry-After"))
			}
			if c.code == codeSlotTaken && (len(p.NearestFreeSlots) != 1 || p.NearestFreeSlots[0].RunwayID != "18-36") {
				t.Errorf("faltan los libres más cercanos: %+v", p.NearestFreeSlots)
			}
		})
	}
}

func TestRequestSlotIsCheckedBeforeCallingPistas(t *testing.T) {
	s := newServer()
	f := s.publish(t, aerolineasArgentinas)
	start := f.ScheduledAt.Truncate(15 * time.Minute)
	path := "/flights/" + f.ID + "/slot-request"

	cases := []struct {
		name    string
		headers map[string]string
		body    map[string]any
		status  int
	}{
		{"bloque fuera de un múltiplo de 15 minutos", aerolineasArgentinas, slotBody("05-23", start.Add(7*time.Minute)), http.StatusBadRequest},
		{"sin pista", aerolineasArgentinas, slotBody("", start), http.StatusBadRequest},
		{"vuelo de otra aerolínea (CA-26)", latam, slotBody("05-23", start), http.StatusForbidden},
	}
	for _, c := range cases {
		if rec := s.do(t, http.MethodPost, path, c.headers, c.body); rec.Code != c.status {
			t.Errorf("%s: got %d %s", c.name, rec.Code, rec.Body.String())
		}
	}

	// Un vuelo cancelado no pide slot (CA-15).
	s.do(t, http.MethodPost, "/flights/"+f.ID+"/cancel", aerolineasArgentinas, nil)
	if rec := s.do(t, http.MethodPost, path, aerolineasArgentinas, slotBody("05-23", start)); rec.Code != http.StatusConflict {
		t.Errorf("cancelado: got %d", rec.Code)
	}
	if len(s.slots.got) != 0 {
		t.Errorf("no se tendría que haber llamado a Pistas: %+v", s.slots.got)
	}
}
