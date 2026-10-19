package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

// Exchange y formato de los mensajes (4.8, 4.10).
const (
	exchangeName  = "aeropuerto.eventos"
	eventVersion  = 1
	publishTimout = 5 * time.Second
)

// message es el formato de cada evento en RabbitMQ (4.10).
type message struct {
	ID            string           `json:"id"`
	Type          string           `json:"type"`
	Version       int              `json:"version"`
	OccurredAt    time.Time        `json:"occurred_at"`
	CorrelationID string           `json:"correlation_id"`
	DataVersion   int64            `json:"data_version"`
	Data          models.EventData `json:"data"`
}

// RabbitPublisher publica eventos en el exchange del sistema. Se conecta cuando
// hace falta y, si la conexión se cae, se vuelve a conectar en el próximo intento.
type RabbitPublisher struct {
	url string

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitPublisher(url string) *RabbitPublisher {
	return &RabbitPublisher{url: url}
}

// Publish publica un evento y espera la confirmación de RabbitMQ. Solo si la
// recibe devuelve nil: recién ahí se puede sacar el evento del outbox.
func (p *RabbitPublisher) Publish(ctx context.Context, e models.PendingEvent) error {
	body, err := json.Marshal(message{
		ID:            e.ID,
		Type:          e.Type,
		Version:       eventVersion,
		OccurredAt:    e.OccurredAt.UTC(),
		CorrelationID: e.CorrelationID,
		DataVersion:   e.DataVersion,
		Data:          e.Data,
	})
	if err != nil {
		return fmt.Errorf("armar el mensaje: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	ch, err := p.ensureChannel()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, publishTimout)
	defer cancel()
	confirmation, err := ch.PublishWithDeferredConfirmWithContext(ctx, exchangeName, e.Type, true, false, amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		MessageId:     e.ID,
		CorrelationId: e.CorrelationID,
		Type:          e.Type,
		Timestamp:     e.OccurredAt,
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
	return nil
}

// ensureChannel abre la conexión y el canal si no están abiertos, declara el
// exchange y activa las confirmaciones de publicación.
func (p *RabbitPublisher) ensureChannel() (*amqp.Channel, error) {
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
	p.conn, p.channel = conn, ch
	return ch, nil
}

// Close cierra la conexión.
func (p *RabbitPublisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *RabbitPublisher) closeLocked() {
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.conn, p.channel = nil, nil
}
