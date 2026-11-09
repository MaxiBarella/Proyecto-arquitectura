package queries

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

type memAll struct{ flights []domain.Flight }

func (m *memAll) ListAfter(_ context.Context, after string, limit int) ([]domain.Flight, error) {
	var out []domain.Flight
	for _, f := range m.flights {
		if f.ID > after && len(out) < limit {
			out = append(out, f)
		}
	}
	return out, nil
}

// Recorre todos los vuelos, de a tandas, y escribe cada uno con su versión y el
// horario del tablero (14.3, 14.6, RN-39).
func TestReindex(t *testing.T) {
	all := &memAll{}
	for i := range 450 { // más de dos tandas
		f := domain.Flight{ID: fmt.Sprintf("v%04d", i), ScheduledAt: published, Version: int64(i + 1), Status: domain.PendingSlot}
		if i == 7 {
			f.Status, f.Slot = domain.Scheduled, &domain.Slot{RunwayID: "05-23", BlockStart: block}
		}
		all.flights = append(all.flights, f)
	}
	idx := &memIndex{}
	n, err := Reindex(context.Background(), all, idx)
	if err != nil || n != 450 || len(idx.docs) != 450 {
		t.Fatalf("escritos %d (%d), err %v", n, len(idx.docs), err)
	}
	if d := idx.docs[7]; !d.BoardAt.Equal(block) || d.DataVersion != 8 || d.Slot == nil {
		t.Errorf("con slot: %+v", d)
	}
	if d := idx.docs[8]; !d.BoardAt.Equal(published) || d.DataVersion != 9 {
		t.Errorf("sin slot: %+v", d)
	}
}

func TestReindexStopsOnIndexError(t *testing.T) {
	all := &memAll{flights: []domain.Flight{{ID: "v1", ScheduledAt: time.Now()}}}
	if _, err := Reindex(context.Background(), all, &memIndex{err: fmt.Errorf("solr caído")}); err == nil {
		t.Error("tiene que fallar")
	}
}
