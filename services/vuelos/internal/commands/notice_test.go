package commands

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// closureHitIn busca el aviso en el outbox.
func closureHitIn(t *testing.T, outbox []domain.Event) (ClosureHitData, bool) {
	t.Helper()
	for _, e := range outbox {
		if e.Type == EventFlightClosureHit {
			return e.Data.(ClosureHitData), true
		}
	}
	return ClosureHitData{}, false
}

// El cierre le deja a la aerolínea un aviso, junto con "vuelo actualizado" (27.1).
func TestClosureLeavesNoticeForDelayedDeparture(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.Scheduled)
	if err := s.HandleEvent(context.Background(), store, msg(EventSlotReleased, slotData(id))); err != nil {
		t.Fatal(err)
	}
	if len(store.outbox) != 2 {
		t.Fatalf("se esperaban vuelo actualizado y el aviso: %+v", store.outbox)
	}
	d, ok := closureHitIn(t, store.outbox)
	if !ok {
		t.Fatal("falta el aviso")
	}
	// La pista y el bloque salen del evento: el despegue demorado ya no tiene slot.
	if d.Status != string(domain.Delayed) || d.Airline != "AR" || d.RunwayID != "05-23" || !d.BlockStart.Equal(block) {
		t.Errorf("aviso: %+v", d)
	}
}

func TestClosureLeavesNoticeForDivertedArrival(t *testing.T) {
	s, store, id := setup(t, domain.Arrival, domain.Scheduled)
	data := slotData(id)
	data["diverted_to"] = "SAOU"
	if err := s.HandleEvent(context.Background(), store, msg(EventSlotReleased, data)); err != nil {
		t.Fatal(err)
	}
	d, ok := closureHitIn(t, store.outbox)
	if !ok || d.Status != string(domain.Diverted) || d.DivertedTo == nil || *d.DivertedTo != "SAOU" {
		t.Errorf("aviso: %+v", d)
	}
}

// Solo el cierre avisa: un slot asignado o cedido no manda mail.
func TestOnlyClosureLeavesNotice(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.PendingSlot)
	_ = s.HandleEvent(context.Background(), store, msg(EventSlotAssigned, slotData(id)))
	if _, ok := closureHitIn(t, store.outbox); ok {
		t.Error("un slot asignado no tiene que dejar aviso")
	}
}

// Un cierre repetido no deja otro aviso: el mensaje se aplica una sola vez (4.9).
func TestRepeatedClosureLeavesOneNotice(t *testing.T) {
	s, store, id := setup(t, domain.Departure, domain.Scheduled)
	m := msg(EventSlotReleased, slotData(id))
	_ = s.HandleEvent(context.Background(), store, m)
	_ = s.HandleEvent(context.Background(), store, m)
	n := 0
	for _, e := range store.outbox {
		if e.Type == EventFlightClosureHit {
			n++
		}
	}
	if n != 1 {
		t.Errorf("avisos: %d", n)
	}
}

type memMailer struct {
	sent []Mail
	err  error
}

func (m *memMailer) Send(_ context.Context, mail Mail) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, mail)
	return nil
}

func noticeMsg(t *testing.T, d ClosureHitData) Message {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return Message{ID: "evento-1", Type: EventFlightClosureHit, Version: 1, Data: raw}
}

var delayed = ClosureHitData{FlightID: "vuelo-1", FlightNumber: "AR1550", Airline: "AR", Type: "departure",
	Status: "delayed", Origin: "Córdoba", Destination: "Buenos Aires", RunwayID: "05-23", BlockStart: block}

func TestNotifierSendsMailToTheAirline(t *testing.T) {
	mailer := &memMailer{}
	n := NewNotifier(mailer, map[string]string{"AR": "ops@ar.test"})
	if err := n.HandleEvent(context.Background(), noticeMsg(t, delayed)); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("mails: %+v", mailer.sent)
	}
	m := mailer.sent[0]
	// El id del evento es la Idempotency-Key: un reintento no manda el mail dos veces (27.6).
	if m.IdempotencyKey != "evento-1" || m.To != "ops@ar.test" {
		t.Errorf("mail: %+v", m)
	}
	// 14:30 UTC son las 11:30 en Argentina.
	if !strings.Contains(m.Subject, "AR1550") || !strings.Contains(m.Subject, "demorado") || !strings.Contains(m.Body, "11:30") {
		t.Errorf("texto: %q / %q", m.Subject, m.Body)
	}
}

func TestNotifierDivertedMentionsTheAlternate(t *testing.T) {
	mailer := &memMailer{}
	n := NewNotifier(mailer, map[string]string{"AR": "ops@ar.test"})
	d := delayed
	alt := "SAOU"
	d.Type, d.Status, d.DivertedTo = "arrival", "diverted", &alt
	if err := n.HandleEvent(context.Background(), noticeMsg(t, d)); err != nil {
		t.Fatal(err)
	}
	if m := mailer.sent[0]; !strings.Contains(m.Subject, "desviado a SAOU") {
		t.Errorf("asunto: %q", m.Subject)
	}
}

// Lo que reintentar no arregla se confirma sin mandar nada (4.22).
func TestNotifierUnprocessable(t *testing.T) {
	n := NewNotifier(&memMailer{}, map[string]string{"AR": "ops@ar.test"})
	other := delayed
	other.Airline = "XX"
	cases := map[string]Message{
		"aerolínea sin mail": noticeMsg(t, other),
		"otro tipo":          {ID: "m", Type: EventFlightUpdated, Data: json.RawMessage(`{}`)},
		"datos rotos":        {ID: "m", Type: EventFlightClosureHit, Data: json.RawMessage(`"x"`)},
	}
	for name, m := range cases {
		if err := n.HandleEvent(context.Background(), m); !errors.Is(err, ErrUnprocessable) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Si el proveedor falla, el error vuelve para que el consumidor reintente (27.5).
func TestNotifierPassesMailerErrors(t *testing.T) {
	fail := errors.New("503 no_disponible")
	n := NewNotifier(&memMailer{err: fail}, map[string]string{"AR": "ops@ar.test"})
	err := n.HandleEvent(context.Background(), noticeMsg(t, delayed))
	if !errors.Is(err, fail) || errors.Is(err, ErrUnprocessable) {
		t.Errorf("error: %v", err)
	}
}
