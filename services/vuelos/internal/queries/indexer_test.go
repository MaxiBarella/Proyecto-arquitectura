package queries

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
)

type memIndex struct {
	docs []IndexedFlight
	err  error
}

func (m *memIndex) Upsert(_ context.Context, f IndexedFlight) error {
	if m.err != nil {
		return m.err
	}
	m.docs = append(m.docs, f)
	return nil
}

var (
	published = time.Date(2026, 11, 10, 14, 30, 0, 0, time.UTC)
	block     = time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)
)

func updated(t *testing.T, d commands.FlightUpdatedData, version int64) commands.Message {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return commands.Message{ID: "m-1", Type: commands.EventFlightUpdated, DataVersion: version, Data: raw}
}

// El horario del tablero es el del bloque si el vuelo tiene slot, y el publicado si no (RN-39).
func TestIndexerBoardTime(t *testing.T) {
	idx := &memIndex{}
	ix := NewIndexer(idx)
	d := commands.FlightUpdatedData{ID: "vuelo-1", FlightNumber: "AR1550", ScheduledAt: published, Status: "pending_slot"}
	if err := ix.HandleEvent(context.Background(), updated(t, d, 1)); err != nil {
		t.Fatal(err)
	}
	d.Status, d.Slot = "scheduled", &commands.FlightSlotData{RunwayID: "05-23", BlockStart: block}
	if err := ix.HandleEvent(context.Background(), updated(t, d, 2)); err != nil {
		t.Fatal(err)
	}
	if len(idx.docs) != 2 || !idx.docs[0].BoardAt.Equal(published) || !idx.docs[1].BoardAt.Equal(block) {
		t.Fatalf("horarios: %+v", idx.docs)
	}
	if idx.docs[1].DataVersion != 2 || idx.docs[1].ID != "vuelo-1" {
		t.Errorf("documento: %+v", idx.docs[1])
	}
}

// Lo que no es un "vuelo actualizado" válido se confirma sin reintentar (4.22).
func TestIndexerUnprocessable(t *testing.T) {
	ix := NewIndexer(&memIndex{})
	cases := map[string]commands.Message{
		"otro tipo":   {ID: "m", Type: commands.EventFlightCancelled, DataVersion: 1, Data: json.RawMessage(`{"flight_id":"x"}`)},
		"sin id":      updated(t, commands.FlightUpdatedData{}, 1),
		"sin versión": updated(t, commands.FlightUpdatedData{ID: "vuelo-1"}, 0),
		"datos rotos": {ID: "m", Type: commands.EventFlightUpdated, DataVersion: 1, Data: json.RawMessage(`"x"`)},
	}
	for name, m := range cases {
		if err := ix.HandleEvent(context.Background(), m); !errors.Is(err, commands.ErrUnprocessable) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Si Solr falla, el error vuelve tal cual para que el consumidor reintente (14.10).
func TestIndexerPassesIndexErrors(t *testing.T) {
	fail := errors.New("solr caído")
	ix := NewIndexer(&memIndex{err: fail})
	err := ix.HandleEvent(context.Background(), updated(t, commands.FlightUpdatedData{ID: "vuelo-1"}, 1))
	if !errors.Is(err, fail) || errors.Is(err, commands.ErrUnprocessable) {
		t.Errorf("error: %v", err)
	}
}
