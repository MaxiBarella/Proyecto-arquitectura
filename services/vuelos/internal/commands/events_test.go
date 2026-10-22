package commands

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// memMessages es una base en memoria con ApplyMessage, que registra cada mensaje
// para no aplicarlo dos veces, como la de MySQL.
type memMessages struct {
	flights   map[string]*domain.Flight
	processed map[string]bool
	err       error
}

func (m *memMessages) ApplyMessage(_ context.Context, messageID, flightID string, change domain.Change) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if m.processed[messageID] {
		return false, nil
	}
	f, ok := m.flights[flightID]
	if !ok {
		return false, domain.ErrNotFound
	}
	copied := *f
	if _, err := change(&copied); err != nil {
		return false, err
	}
	m.flights[flightID] = &copied
	m.processed[messageID] = true
	return true, nil
}

var (
	now   = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	block = time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)
)

func setup(t *testing.T, flightType domain.FlightType, status domain.Status) (*Service, *memMessages, string) {
	t.Helper()
	d := domain.NewFlightData{Number: "AR1550", Type: flightType, Scope: domain.Domestic, ScheduledAt: block,
		Origin: "Córdoba", Destination: "Buenos Aires"}
	if flightType == domain.Arrival {
		d.Origin, d.Destination = "Santiago", "Córdoba"
	}
	f, err := domain.NewFlight("vuelo-1", "AR", d, now)
	if err != nil {
		t.Fatal(err)
	}
	f.Status = status
	if status == domain.Scheduled {
		f.Slot = &domain.Slot{RunwayID: "05-23", BlockStart: block}
	}
	store := &memMessages{flights: map[string]*domain.Flight{f.ID: f}, processed: map[string]bool{}}
	s := New(nil, nil)
	s.now = func() time.Time { return now }
	return s, store, f.ID
}

func msg(eventType string, data map[string]any) Message {
	raw, _ := json.Marshal(data)
	return Message{ID: domain.NewID(), Type: eventType, Version: 1, Data: raw}
}

func slotData(flightID string) map[string]any {
	return map[string]any{"flight_id": flightID, "runway_id": "05-23", "block_start": block}
}

func TestSlotAssigned(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.PendingSlot)
	if err := s.HandleEvent(context.Background(), store, msg(EventSlotAssigned, slotData(id))); err != nil {
		t.Fatal(err)
	}
	f := store.flights[id]
	if f.Status != domain.Scheduled || f.Slot == nil || f.Slot.RunwayID != "05-23" || !f.Slot.BlockStart.Equal(block) {
		t.Errorf("tiene que quedar programado con el slot (CA-02): %+v", f)
	}
}

func TestSlotCeded(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.Scheduled)
	data := slotData(id)
	data["taken_by_flight_id"] = "arribo-1"
	if err := s.HandleEvent(context.Background(), store, msg(EventSlotCeded, data)); err != nil {
		t.Fatal(err)
	}
	if f := store.flights[id]; f.Status != domain.Delayed || f.Slot != nil {
		t.Errorf("el despegue tiene que quedar demorado y sin slot (CA-07): %+v", f)
	}
}

func TestSlotReleasedByClosure(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.Scheduled)
	data := slotData(id)
	data["diverted_to"] = nil
	if err := s.HandleEvent(context.Background(), store, msg(EventSlotReleased, data)); err != nil {
		t.Fatal(err)
	}
	if f := store.flights[id]; f.Status != domain.Delayed {
		t.Errorf("despegue: tiene que quedar demorado (CA-12): %s", f.Status)
	}

	s, store, id = setup(t, domain.Arrival, domain.Scheduled)
	data = slotData(id)
	data["diverted_to"] = "SAOU"
	if err := s.HandleEvent(context.Background(), store, msg(EventSlotReleased, data)); err != nil {
		t.Fatal(err)
	}
	if f := store.flights[id]; f.Status != domain.Diverted || f.DivertedTo != "SAOU" {
		t.Errorf("arribo: tiene que quedar desviado a SAOU (CA-13): %+v", f)
	}
}

func TestTheSameMessageIsAppliedOnce(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.PendingSlot)
	m := msg(EventSlotAssigned, slotData(id))
	for range 2 {
		if err := s.HandleEvent(context.Background(), store, m); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.processed) != 1 {
		t.Errorf("registrados %d mensajes", len(store.processed))
	}
}

// Eventos que no se pueden aplicar: se confirman sin reintentar (4.22).
func TestUnprocessableEvents(t *testing.T) {
	cases := []struct {
		name   string
		status domain.Status
		m      func(id string) Message
	}{
		{"slot asignado para un vuelo cancelado", domain.Cancelled, func(id string) Message { return msg(EventSlotAssigned, slotData(id)) }},
		{"vuelo que no existe", domain.PendingSlot, func(string) Message { return msg(EventSlotAssigned, slotData("otro")) }},
		{"sin flight_id", domain.PendingSlot, func(string) Message { return msg(EventSlotAssigned, map[string]any{}) }},
		{"arribo desviado sin alternativo", domain.Scheduled, func(id string) Message { return msg(EventSlotReleased, slotData(id)) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			flightType := domain.Departure
			if c.name == "arribo desviado sin alternativo" {
				flightType = domain.Arrival
			}
			s, store, id := setup(t, flightType, c.status)
			err := s.HandleEvent(context.Background(), store, c.m(id))
			if !errors.Is(err, ErrUnprocessable) {
				t.Errorf("got %v, want ErrUnprocessable", err)
			}
		})
	}
}

func TestEventsWithoutEffectAreConfirmed(t *testing.T) {
	s, store, _ := setup(t, domain.Departure, domain.PendingSlot)
	for _, eventType := range []string{EventRunwayClosed, EventRunwayReopened, "pistas.algo.nuevo"} {
		if err := s.HandleEvent(context.Background(), store, msg(eventType, map[string]any{"runway_id": "05-23"})); err != nil {
			t.Errorf("%s: %v", eventType, err)
		}
	}
}

// Una falla de la base se devuelve tal cual, para que el consumidor reintente.
func TestTransientErrorsAreReturned(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.PendingSlot)
	store.err = errors.New("MySQL no responde")
	err := s.HandleEvent(context.Background(), store, msg(EventSlotAssigned, slotData(id)))
	if err == nil || errors.Is(err, ErrUnprocessable) {
		t.Errorf("got %v", err)
	}
}
