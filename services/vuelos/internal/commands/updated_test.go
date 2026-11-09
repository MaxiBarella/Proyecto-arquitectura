package commands

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// Cada evento de Pistas que cambia un vuelo deja "vuelo actualizado" con el vuelo
// ya cambiado, para el índice del tablero (14.5).
func TestEventsLeaveFlightUpdated(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.PendingSlot)
	if err := s.HandleEvent(context.Background(), store, msg(EventSlotAssigned, slotData(id))); err != nil {
		t.Fatal(err)
	}
	if len(store.outbox) != 1 || store.outbox[0].Type != EventFlightUpdated {
		t.Fatalf("outbox: %+v", store.outbox)
	}
	d, ok := store.outbox[0].Data.(FlightUpdatedData)
	if !ok || d.ID != id || d.Status != string(domain.Scheduled) || d.Slot == nil || d.Slot.RunwayID != "05-23" {
		t.Errorf("datos: %+v", store.outbox[0].Data)
	}
}

func TestDivertedArrivalCarriesAlternate(t *testing.T) {
	s, store, id := setup(t, domain.Arrival, domain.Scheduled)
	data := slotData(id)
	data["diverted_to"] = "SAOU"
	if err := s.HandleEvent(context.Background(), store, msg(EventSlotReleased, data)); err != nil {
		t.Fatal(err)
	}
	d := store.outbox[0].Data.(FlightUpdatedData)
	if d.Status != string(domain.Diverted) || d.DivertedTo == nil || *d.DivertedTo != "SAOU" {
		t.Errorf("datos: %+v", d)
	}
}

// Un evento repetido o que no se puede aplicar no deja "vuelo actualizado".
func TestNoFlightUpdatedWithoutChange(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.Cancelled)
	_ = s.HandleEvent(context.Background(), store, msg(EventSlotAssigned, slotData(id)))
	if len(store.outbox) != 0 {
		t.Errorf("no tiene que dejar eventos: %+v", store.outbox)
	}

	s, store, id = setup(t, domain.Departure, domain.PendingSlot)
	m := msg(EventSlotAssigned, slotData(id))
	_ = s.HandleEvent(context.Background(), store, m)
	_ = s.HandleEvent(context.Background(), store, m)
	if len(store.outbox) != 1 {
		t.Errorf("el repetido no tiene que dejar otro: %d", len(store.outbox))
	}
}

// Los datos usan los nombres del contrato de Vuelos; slot y diverted_to van en null
// si el vuelo no los tiene.
func TestFlightUpdatedDataJSON(t *testing.T) {
	_, store, id := setup(t, domain.Departure, domain.PendingSlot)
	raw, err := json.Marshal(NewFlightUpdatedData(store.flights[id]))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	for _, k := range []string{"id", "flight_number", "airline", "type", "scope", "origin", "destination", "scheduled_at", "status", "slot", "diverted_to", "created_at", "updated_at"} {
		if _, ok := got[k]; !ok {
			t.Errorf("falta %q en %s", k, raw)
		}
	}
	if got["slot"] != nil || got["diverted_to"] != nil {
		t.Errorf("slot y diverted_to tienen que ir en null: %s", raw)
	}
}
