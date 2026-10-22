package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// acks guarda qué se hizo con el mensaje.
type acks struct{ result string }

func (a *acks) Ack(uint64, bool) error { a.result = "ack"; return nil }
func (a *acks) Nack(_ uint64, _ bool, requeue bool) error {
	a.result = fmt.Sprintf("nack requeue=%v", requeue)
	return nil
}
func (a *acks) Reject(uint64, bool) error { a.result = "reject"; return nil }

func delivery(t *testing.T, a *acks, body any) amqp.Delivery {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return amqp.Delivery{Acknowledger: a, Body: raw, RoutingKey: "pistas.slot.asignado", MessageId: "m1"}
}

var validMessage = map[string]any{"id": "m1", "type": "pistas.slot.asignado", "version": 1, "data": map[string]any{}}

func newConsumer(handler Handler) *Consumer {
	c := NewConsumer("amqp://no-se-usa", handler)
	c.retryWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	return c
}

func TestProcess(t *testing.T) {
	cases := []struct {
		name     string
		handler  Handler
		body     any
		want     string
		attempts int
	}{
		{"aplicado", func(context.Context, commands.Message) error { return nil }, validMessage, "ack", 1},
		{"no se puede aplicar (4.22)", func(context.Context, commands.Message) error { return commands.ErrUnprocessable }, validMessage, "ack", 1},
		{"falla pasajera: 3 reintentos y a fallidos (4.5, 4.20)", func(context.Context, commands.Message) error { return errors.New("MySQL no responde") }, validMessage, "nack requeue=false", 4},
		{"mensaje que no es JSON", func(context.Context, commands.Message) error { return nil }, "no es un mensaje", "ack", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			attempts := 0
			consumer := newConsumer(func(ctx context.Context, m commands.Message) error {
				attempts++
				return c.handler(ctx, m)
			})
			a := &acks{}
			consumer.process(context.Background(), delivery(t, a, c.body))
			if a.result != c.want || attempts != c.attempts {
				t.Errorf("got %s con %d intentos, want %s con %d", a.result, attempts, c.want, c.attempts)
			}
		})
	}
}

func TestProcessRecoversAfterARetry(t *testing.T) {
	attempts := 0
	consumer := newConsumer(func(context.Context, commands.Message) error {
		attempts++
		if attempts < 3 {
			return errors.New("MySQL no responde")
		}
		return nil
	})
	a := &acks{}
	consumer.process(context.Background(), delivery(t, a, validMessage))
	if a.result != "ack" || attempts != 3 {
		t.Errorf("got %s con %d intentos", a.result, attempts)
	}
}

// Si Vuelos se apaga en medio de un mensaje, vuelve a la cola: no va a fallidos.
func TestProcessRequeuesOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	consumer := newConsumer(func(context.Context, commands.Message) error {
		cancel()
		return errors.New("MySQL no responde")
	})
	a := &acks{}
	consumer.process(ctx, delivery(t, a, validMessage))
	if a.result != "nack requeue=true" {
		t.Errorf("got %s", a.result)
	}
}

// El mensaje tiene el formato de la 4.10, con los datos tal cual salen del outbox.
func TestEncodeMessage(t *testing.T) {
	body, err := encodeMessage(domain.Event{
		ID:            "e1",
		Type:          "vuelos.vuelo.cancelado",
		OccurredAt:    time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC),
		CorrelationID: "c1",
		DataVersion:   2,
		Data:          json.RawMessage(`{"flight_id": "f1"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"e1","type":"vuelos.vuelo.cancelado","version":1,"occurred_at":"2026-10-22T12:00:00Z","correlation_id":"c1","data_version":2,"data":{"flight_id":"f1"}}`
	if string(body) != want {
		t.Errorf("got  %s\nwant %s", body, want)
	}
}
