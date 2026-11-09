package commands

import (
	"context"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// memFlights es una base en memoria con Update y DueForCompletion.
type memFlights struct {
	flights map[string]*domain.Flight
	due     []string
	outbox  []domain.Event
}

func (m *memFlights) Create(context.Context, *domain.Flight, ...domain.Event) error { return nil }
func (m *memFlights) Get(_ context.Context, id string) (*domain.Flight, error) {
	return m.flights[id], nil
}
func (m *memFlights) Update(_ context.Context, id string, change domain.Change) (*domain.Flight, error) {
	f, ok := m.flights[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *f
	events, err := change(&copied)
	if err != nil {
		return nil, err
	}
	m.flights[id] = &copied
	m.outbox = append(m.outbox, events...)
	return &copied, nil
}
func (m *memFlights) DueForCompletion(context.Context, time.Time, int) ([]string, error) {
	return m.due, nil
}

func TestCompleteDue(t *testing.T) {
	_, msgs, id := setup(t, domain.Departure, domain.Scheduled) // bloque a las 14:30 del 23/10
	store := &memFlights{flights: msgs.flights}
	// Otro que ya lo finalizó la otra instancia, y uno que no existe.
	_, other, otherID := setup(t, domain.Departure, domain.Completed)
	store.flights[otherID+"-b"] = other.flights[otherID]
	store.due = []string{id, otherID + "-b", "no-existe"}

	s := New(store, nil)
	s.now = func() time.Time { return block.Add(20 * time.Minute) } // 14:50: ya terminó
	n, err := s.CompleteDue(context.Background(), store)
	if err != nil || n != 1 {
		t.Fatalf("finalizados %d, err %v", n, err)
	}
	if store.flights[id].Status != domain.Completed {
		t.Errorf("estado: %s", store.flights[id].Status)
	}
	// Deja "vuelo actualizado" para el tablero (14.5).
	if len(store.outbox) != 1 || store.outbox[0].Type != EventFlightUpdated ||
		store.outbox[0].Data.(FlightUpdatedData).Status != string(domain.Completed) {
		t.Errorf("outbox: %+v", store.outbox)
	}
}

// Si el bloque todavía no terminó, no se finaliza.
func TestCompleteDueWaitsForTheBlock(t *testing.T) {
	_, msgs, id := setup(t, domain.Departure, domain.Scheduled)
	store := &memFlights{flights: msgs.flights, due: []string{id}}
	s := New(store, nil)
	s.now = func() time.Time { return block.Add(10 * time.Minute) } // 14:40: sigue
	if n, err := s.CompleteDue(context.Background(), store); err != nil || n != 0 {
		t.Fatalf("finalizados %d, err %v", n, err)
	}
	if store.flights[id].Status != domain.Scheduled {
		t.Errorf("estado: %s", store.flights[id].Status)
	}
}
