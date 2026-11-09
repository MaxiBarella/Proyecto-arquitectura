package mysql

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// Solo los programados cuyo bloque ya terminó (RN-32).
func TestDueForCompletion(t *testing.T) {
	r := newRepo(t)
	create := func(number string, status domain.Status, slotAt *time.Time) string {
		f := newFlight(t, "AR", number, block)
		f.Status = status
		if slotAt != nil {
			f.Slot = &domain.Slot{RunwayID: "05-23", BlockStart: *slotAt}
		}
		if err := r.Create(ctx, f); err != nil {
			t.Fatalf("Create: %v", err)
		}
		return f.ID
	}
	ended := block                         // terminó a las 14:45
	running := block.Add(30 * time.Minute) // 15:00 a 15:15
	due := create("AR1", domain.Scheduled, &ended)
	create("AR2", domain.Scheduled, &running)
	create("AR3", domain.Delayed, nil)
	create("AR4", domain.PendingSlot, nil)

	ids, err := r.DueForCompletion(ctx, block.Add(40*time.Minute), 10) // 15:10
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != due {
		t.Errorf("got %v, want solo %s", ids, due)
	}
}

// ListAfter recorre todos los vuelos de a tandas, sin repetir ni saltear.
func TestListAfter(t *testing.T) {
	r := newRepo(t)
	var want []string
	for i := range 7 {
		f := newFlight(t, "AR", fmt.Sprintf("AR%d", 100+i), block)
		if err := r.Create(ctx, f); err != nil {
			t.Fatalf("Create: %v", err)
		}
		want = append(want, f.ID)
	}
	sort.Strings(want)
	var got []string
	after := ""
	for {
		batch, err := r.ListAfter(ctx, after, 3)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range batch {
			got = append(got, f.ID)
		}
		if len(batch) < 3 {
			break
		}
		after = batch[len(batch)-1].ID
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
