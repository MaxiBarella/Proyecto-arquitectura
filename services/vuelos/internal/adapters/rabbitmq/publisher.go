package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/observability"
)

// Exchange y formato de los mensajes (4.8, 4.10). Es el mismo que usa Clima.
const (
	exchangeName   = "aeropuerto.eventos"
	eventVersion   = 1
	publishTimeout = 5 * time.Second
)

// ErrNoQueue indica que RabbitMQ devolvió el evento porque ninguna cola lo recibe.
// El evento se queda en el outbox hasta la próxima vuelta.
var ErrNoQueue = errors.New("ninguna cola recibe el evento todavía")

// message es el formato de cada evento en RabbitMQ (4.10).
type message struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Version       int       `json:"version"`
	OccurredAt    time.Time `json:"occurred_at"`
	CorrelationID string    `json:"correlation_id"`
	DataVersion   int64     `json:"data_version"`
	Data          any       `json:"data"`
}

func encodeMessage(e domain.Event) ([]byte, error) {
	return json.Marshal(message{
		ID:            e.ID,
		Type:          e.Type,
		Version:       eventVersion,
		OccurredAt:    e.OccurredAt.UTC(),
		CorrelationID: e.CorrelationID,
		DataVersion:   e.DataVersion,
		Data:          e.Data,
	})
}

// Publisher publica eventos en el exchange del sistema. Se conecta cuando hace
// falta y, si la conexión se cae, se vuelve a conectar en el próximo intento.
type Publisher struct {
	url string

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
	returns chan amqp.Return
}

func NewPublisher(url string) *Publisher {
	return &Publisher{url: url}
}

// Publish publica un evento y espera la confirmación de RabbitMQ. Solo devuelve nil
// si RabbitMQ lo confirmó y lo dejó en al menos una cola: recién ahí el relay lo
// borra del outbox. La traza viaja en el encabezado traceparent (11.6).
func (p *Publisher) Publish(ctx context.Context, e domain.Event) error {
	body, err := encodeMessage(e)
	if err != nil {
		return fmt.Errorf("armar el mensaje: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	ch, err := p.ensureChannel()
	if err != nil {
		return err
	}
	p.discardOldReturns()

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	// mandatory=true: si ninguna cola lo recibe, RabbitMQ lo devuelve en vez de descartarlo.
	confirmation, err := ch.PublishWithDeferredConfirmWithContext(ctx, exchangeName, e.Type, true, false, amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		MessageId:     e.ID,
		CorrelationId: e.CorrelationID,
		Type:          e.Type,
		Timestamp:     e.OccurredAt,
		Headers:       amqp.Table(observability.MessageHeaders(ctx)),
		Body:          body,
	})
	if err != nil {
		p.closeLocked()
		return fmt.Errorf("publicar en RabbitMQ: %w", err)
	}
	ok, err := confirmation.WaitContext(ctx)
	if err != nil {
		p.closeLocked()
		return fmt.Errorf("esperar la confirmación de RabbitMQ: %w", err)
	}
	if !ok {
		return errors.New("RabbitMQ rechazó el mensaje")
	}
	// RabbitMQ manda la devolución antes que la confirmación, así que si hubo una, ya llegó.
	select {
	case r := <-p.returns:
		if r.MessageId == e.ID {
			return fmt.Errorf("%w (%s)", ErrNoQueue, e.Type)
		}
	default:
	}
	return nil
}

func (p *Publisher) discardOldReturns() {
	for {
		select {
		case _, open := <-p.returns:
			if !open {
				return
			}
		default:
			return
		}
	}
}

// ensureChannel abre la conexión y el canal si no están abiertos, declara el
// exchange igual que en rabbitmq/definitions.json y activa las confirmaciones.
func (p *Publisher) ensureChannel() (*amqp.Channel, error) {
	if p.channel != nil && !p.channel.IsClosed() {
		return p.channel, nil
	}
	p.closeLocked()

	conn, err := amqp.Dial(p.url)
	if err != nil {
		return nil, fmt.Errorf("conectar con RabbitMQ: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("abrir un canal de RabbitMQ: %w", err)
	}
	if err := ch.ExchangeDeclare(exchangeName, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("declarar el exchange: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("activar las confirmaciones: %w", err)
	}
	p.returns = ch.NotifyReturn(make(chan amqp.Return, 16))
	p.conn, p.channel = conn, ch
	return ch, nil
}

// Close cierra la conexión.
func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *Publisher) closeLocked() {
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.conn, p.channel, p.returns = nil, nil, nil
}
