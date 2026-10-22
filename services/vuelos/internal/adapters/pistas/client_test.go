package pistas

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

var (
	block   = time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)
	request = commands.SlotRequest{
		FlightID:   "7f1c2a3e-5b6d-4e8f-9a0b-1c2d3e4f5a6b",
		FlightType: domain.Departure,
		Slot:       domain.Slot{RunwayID: "05-23", BlockStart: block},
	}
)

// fakePistas responde con handler y cuenta los pedidos que recibe.
type fakePistas struct {
	*httptest.Server
	calls atomic.Int32
}

func newFakePistas(t *testing.T, handler http.HandlerFunc) *fakePistas {
	t.Helper()
	f := &fakePistas{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

// Los mismos valores que DefaultConfig, pero con tiempos cortos para el test.
func newTestClient(url string) *Client {
	cfg := DefaultConfig(url)
	cfg.AttemptTimeout = 100 * time.Millisecond
	cfg.FirstBackoff = time.Millisecond
	cfg.BreakerOpenFor = 200 * time.Millisecond
	return NewClient(cfg)
}

func assigned(w http.ResponseWriter, r *http.Request) {
	var body slotRequestBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"flight_id": body.FlightID, "runway_id": body.RunwayID, "block_start": body.BlockStart, "displaced_flight_id": nil,
	})
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

func problemResponse(code string, extra map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		body := map[string]any{"code": code, "status": http.StatusConflict}
		for k, v := range extra {
			body[k] = v
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(body)
	}
}

func TestAssigned(t *testing.T) {
	var got slotRequestBody
	pistas := newFakePistas(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/slots" {
			t.Errorf("pedido a %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{"flight_id": got.FlightID, "runway_id": got.RunwayID, "block_start": got.BlockStart})
	})
	slot, err := newTestClient(pistas.URL).RequestSlot(context.Background(), request)
	if err != nil || slot != request.Slot {
		t.Fatalf("got %+v, %v", slot, err)
	}
	// El cuerpo del pedido es el de services/pistas/openapi.yaml.
	if got.FlightID != request.FlightID || got.FlightType != "departure" || got.RunwayID != "05-23" || !got.BlockStart.Equal(block) {
		t.Errorf("cuerpo enviado: %+v", got)
	}
}

func TestRejectionsAreNotRetried(t *testing.T) {
	nearest := []map[string]any{{"runway_id": "18-36", "block_start": block}}
	cases := []struct {
		name    string
		handler http.HandlerFunc
		check   func(error) bool
	}{
		{"slot ocupado", problemResponse("SLOT_TAKEN", map[string]any{"nearest_free_slots": nearest}), func(err error) bool {
			var taken *commands.SlotTakenError
			return errors.As(err, &taken) && len(taken.Nearest) == 1 && taken.Nearest[0].RunwayID == "18-36"
		}},
		{"pista cerrada", problemResponse("RUNWAY_CLOSED", nil), func(err error) bool { return errors.Is(err, commands.ErrRunwayClosed) }},
		{"el vuelo ya tiene slot", problemResponse("FLIGHT_ALREADY_HAS_SLOT", nil), func(err error) bool { return errors.Is(err, domain.ErrInvalidTransition) }},
		{"pista inexistente", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "RUNWAY_NOT_FOUND"})
		}, func(err error) bool { return errors.Is(err, commands.ErrRunwayNotFound) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pistas := newFakePistas(t, c.handler)
			_, err := newTestClient(pistas.URL).RequestSlot(context.Background(), request)
			if !c.check(err) {
				t.Errorf("error: %v", err)
			}
			if n := pistas.calls.Load(); n != 1 {
				t.Errorf("un rechazo no se reintenta (4.5): %d pedidos", n)
			}
		})
	}
}

func TestTransientFailuresAreRetried(t *testing.T) {
	pistas := newFakePistas(t, nil)
	pistas.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pistas.calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		assigned(w, r)
	})
	slot, err := newTestClient(pistas.URL).RequestSlot(context.Background(), request)
	if err != nil || slot != request.Slot || pistas.calls.Load() != 2 {
		t.Errorf("tenía que andar al segundo intento: %+v, %v, %d pedidos", slot, err, pistas.calls.Load())
	}
}

func TestGivesUpAfterTwoRetries(t *testing.T) {
	for _, code := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		pistas := newFakePistas(t, status(code))
		_, err := newTestClient(pistas.URL).RequestSlot(context.Background(), request)
		if !errors.Is(err, commands.ErrRunwaysUnavailable) || pistas.calls.Load() != 3 {
			t.Errorf("%d: tenían que ser 3 intentos (1 + 2 reintentos): %d, %v", code, pistas.calls.Load(), err)
		}
	}
}

func TestOtherServerErrorsAreNotRetried(t *testing.T) {
	pistas := newFakePistas(t, status(http.StatusInternalServerError))
	_, err := newTestClient(pistas.URL).RequestSlot(context.Background(), request)
	if !errors.Is(err, commands.ErrRunwaysUnavailable) || pistas.calls.Load() != 1 {
		t.Errorf("un 500 no se reintenta (4.5): %d pedidos, %v", pistas.calls.Load(), err)
	}
}

// Cada intento tiene su propio plazo (4.16): si Pistas no contesta, se corta y se
// reintenta.
func TestEachAttemptHasItsOwnTimeout(t *testing.T) {
	pistas := newFakePistas(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(time.Second):
		case <-r.Context().Done():
		}
	})
	start := time.Now()
	_, err := newTestClient(pistas.URL).RequestSlot(context.Background(), request)
	elapsed := time.Since(start)
	if !errors.Is(err, commands.ErrRunwaysUnavailable) || pistas.calls.Load() != 3 {
		t.Errorf("tenían que ser 3 intentos cortados por el plazo: %d, %v", pistas.calls.Load(), err)
	}
	if elapsed < 300*time.Millisecond || elapsed > 900*time.Millisecond {
		t.Errorf("tardó %v: tendrían que ser 3 plazos de 100 ms más las esperas", elapsed)
	}
}

func TestBreakerOpensAfterFiveFailures(t *testing.T) {
	var down atomic.Bool
	down.Store(true)
	pistas := newFakePistas(t, func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		assigned(w, r)
	})
	client := newTestClient(pistas.URL)
	ctx := context.Background()

	for i := range 5 {
		if _, err := client.RequestSlot(ctx, request); !errors.Is(err, commands.ErrRunwaysUnavailable) {
			t.Fatalf("pedido %d: %v", i+1, err)
		}
	}
	if client.State() != gobreaker.StateOpen {
		t.Fatalf("después de 5 fallas seguidas tiene que estar abierto, está %s", client.State())
	}

	// Abierto: responde enseguida, sin llamar a Pistas.
	before := pistas.calls.Load()
	start := time.Now()
	if _, err := client.RequestSlot(ctx, request); !errors.Is(err, commands.ErrRunwaysUnavailable) {
		t.Errorf("abierto: %v", err)
	}
	if pistas.calls.Load() != before || time.Since(start) > 50*time.Millisecond {
		t.Errorf("abierto no tendría que llamar a Pistas")
	}

	// Pistas vuelve. Pasado el tiempo abierto, un pedido de prueba lo cierra.
	down.Store(false)
	time.Sleep(250 * time.Millisecond)
	if _, err := client.RequestSlot(ctx, request); err != nil {
		t.Fatalf("pedido de prueba: %v", err)
	}
	if client.State() != gobreaker.StateClosed {
		t.Errorf("con el pedido de prueba bien, se tiene que cerrar: está %s", client.State())
	}
}

// Un rechazo es una respuesta correcta de Pistas: no abre el circuit breaker.
func TestRejectionsDoNotOpenTheBreaker(t *testing.T) {
	pistas := newFakePistas(t, problemResponse("SLOT_TAKEN", nil))
	client := newTestClient(pistas.URL)
	for range 10 {
		_, _ = client.RequestSlot(context.Background(), request)
	}
	if client.State() != gobreaker.StateClosed || pistas.calls.Load() != 10 {
		t.Errorf("estado %s con %d pedidos", client.State(), pistas.calls.Load())
	}
}

// El pedido lleva el encabezado traceparent: la traza sigue en Pistas (11.2).
func TestPropagatesTheTrace(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	var traceparent string
	pistas := newFakePistas(t, func(w http.ResponseWriter, r *http.Request) {
		traceparent = r.Header.Get("traceparent")
		assigned(w, r)
	})
	ctx, span := sdktrace.NewTracerProvider().Tracer("test").Start(context.Background(), "pedir slot")
	defer span.End()

	if _, err := newTestClient(pistas.URL).RequestSlot(ctx, request); err != nil {
		t.Fatal(err)
	}
	if want := span.SpanContext().TraceID().String(); len(traceparent) < 35 || traceparent[3:35] != want {
		t.Errorf("traceparent = %q, se esperaba la traza %s", traceparent, want)
	}
}
