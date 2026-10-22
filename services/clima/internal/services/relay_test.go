package services

import (
	"context"
	"errors"
	"testing"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

type fakeOutbox struct {
	states []models.AptitudeState
	marked []string
}

func (f *fakeOutbox) PendingEvents(context.Context) ([]models.AptitudeState, error) {
	return f.states, nil
}

func (f *fakeOutbox) MarkPublished(_ context.Context, _, id string) error {
	f.marked = append(f.marked, id)
	return nil
}

// fakePublisher falla a partir del evento failFrom (contando desde 1); 0 es nunca.
type fakePublisher struct {
	published []string
	failFrom  int
}

func (f *fakePublisher) Publish(_ context.Context, e models.PendingEvent) error {
	if f.failFrom > 0 && len(f.published)+1 >= f.failFrom {
		return errors.New("RabbitMQ no responde")
	}
	f.published = append(f.published, e.ID)
	return nil
}

func outboxWith(ids ...string) *fakeOutbox {
	st := models.AptitudeState{Airport: "SACO"}
	for _, id := range ids {
		st.Outbox = append(st.Outbox, models.PendingEvent{ID: id, Type: models.EventAptitudeChanged})
	}
	return &fakeOutbox{states: []models.AptitudeState{st}}
}

func TestRelayPublicaEnOrdenYVaciaElOutbox(t *testing.T) {
	outbox, pub := outboxWith("a", "b", "c"), &fakePublisher{}
	if err := NewRelay(outbox, pub).PublishPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, "publicados", pub.published, "a", "b", "c")
	assertIDs(t, "sacados del outbox", outbox.marked, "a", "b", "c")
}

func TestRelaySeDetieneEnElPrimeroQueFalla(t *testing.T) {
	outbox, pub := outboxWith("a", "b", "c"), &fakePublisher{failFrom: 2}
	if err := NewRelay(outbox, pub).PublishPending(context.Background()); err == nil {
		t.Fatal("se esperaba un error")
	}
	// "b" falló: no se publica "c" antes que él, y solo "a" sale del outbox.
	assertIDs(t, "publicados", pub.published, "a")
	assertIDs(t, "sacados del outbox", outbox.marked, "a")
}

func assertIDs(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, se esperaba %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, se esperaba %v", what, got, want)
		}
	}
}
