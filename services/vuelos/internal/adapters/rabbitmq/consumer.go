package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/observability"
)

// QueueName es la cola de Vuelos (4.18). La crea rabbitmq/definitions.json: el
// consumidor no la declara, para no chocar con sus argumentos.
const QueueName = "vuelos.eventos"

// DefaultRetryWaits son las esperas entre intentos ante una falla pasajera: hasta
// 3 reintentos (4.5), con 1, 2 y 4 segundos (4.24).
var DefaultRetryWaits = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// reconnectWait es cuánto se espera para volver a conectarse si se cae RabbitMQ.
const reconnectWait = 5 * time.Second

// Handler procesa un mensaje. Si devuelve commands.ErrUnprocessable, el mensaje
// se confirma igual; cualquier otro error se reintenta.
type Handler func(ctx context.Context, m commands.Message) error

// Consumer lee la cola de Vuelos. La cola tiene un solo consumidor activo (4.23):
// si corren dos instancias, una procesa y la otra espera de reserva, así los
// eventos de un mismo vuelo se aplican en orden.
type Consumer struct {
	url        string
	handler    Handler
	retryWaits []time.Duration
}

func NewConsumer(url string, handler Handler) *Consumer {
	return &Consumer{url: url, handler: handler, retryWaits: DefaultRetryWaits}
}

// Run consume hasta que se cancele ctx. Si la conexión se cae, se vuelve a conectar.
func (c *Consumer) Run(ctx context.Context) {
	for {
		err := c.consume(ctx)
		if ctx.Err() != nil {
			return
		}
		slog.ErrorContext(ctx, "se cortó el consumo de eventos; se vuelve a conectar", "queue", QueueName, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectWait):
		}
	}
}

func (c *Consumer) consume(ctx context.Context) error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("conectar con RabbitMQ: %w", err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("abrir un canal: %w", err)
	}
	// De a un mensaje: el siguiente llega recién cuando se confirma el anterior.
	if err := ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("configurar el canal: %w", err)
	}
	deliveries, err := ch.ConsumeWithContext(ctx, QueueName, "vuelos", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consumir %s: %w", QueueName, err)
	}
	slog.InfoContext(ctx, "consumiendo eventos", "queue", QueueName)
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, open := <-deliveries:
			if !open {
				return errors.New("RabbitMQ cerró la entrega de mensajes")
			}
			c.process(ctx, d)
		}
	}
}

// process aplica un mensaje y lo confirma, o lo manda a fallidos si después de
// los reintentos sigue fallando (4.5, 4.20).
func (c *Consumer) process(ctx context.Context, d amqp.Delivery) {
	ctx, span := observability.StartConsume(ctx, d.Headers, d.RoutingKey)
	m, err := c.handle(ctx, d)
	observability.EndSpan(span, err)
	// Los identificadores salen del cuerpo (4.10); si no se pudo leer, de las propiedades.
	eventID, correlationID := m.ID, m.CorrelationID
	if eventID == "" {
		eventID, correlationID = d.MessageId, d.CorrelationId
	}

	switch {
	case err == nil:
		_ = d.Ack(false)
	case ctx.Err() != nil:
		// Vuelos se está apagando: el mensaje vuelve a la cola y lo toma la otra instancia.
		_ = d.Nack(false, true)
	case errors.Is(err, commands.ErrUnprocessable):
		// Reintentar no lo arregla: se confirma y queda el aviso (4.22).
		slog.WarnContext(ctx, "evento descartado", "event", d.RoutingKey, "event_id", eventID,
			"correlation_id", correlationID, "error", err)
		_ = d.Ack(false)
	default:
		// Sin volver a encolar: RabbitMQ lo pasa solo a vuelos.eventos.fallidos (4.20).
		// Este log es el que dispara la alerta de mensajes fallidos (11.4).
		slog.ErrorContext(ctx, "evento enviado a fallidos", "event", d.RoutingKey, "event_id", eventID,
			"correlation_id", correlationID, "error", err)
		_ = d.Nack(false, false)
	}
}

// handle decodifica el mensaje y lo aplica, reintentando las fallas pasajeras.
func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) (commands.Message, error) {
	var m commands.Message
	if err := json.Unmarshal(d.Body, &m); err != nil || m.ID == "" {
		return m, fmt.Errorf("%w: mensaje inválido: %v", commands.ErrUnprocessable, err)
	}
	for attempt := 0; ; attempt++ {
		err := c.handler(ctx, m)
		if err == nil || errors.Is(err, commands.ErrUnprocessable) || attempt == len(c.retryWaits) {
			return m, err
		}
		slog.WarnContext(ctx, "reintento de un evento", "event", m.Type, "event_id", m.ID, "attempt", attempt+1, "error", err)
		select {
		case <-ctx.Done():
			return m, ctx.Err()
		case <-time.After(c.retryWaits[attempt]):
		}
	}
}
