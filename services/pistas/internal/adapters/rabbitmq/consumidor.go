package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/observability"
)

// Cola es la cola de Pistas (4.18). La crea rabbitmq/definitions.json: el consumidor no la declara,
// para no chocar con sus argumentos.
const Cola = "pistas.eventos"

// EsperasPorDefecto son las esperas entre intentos ante una falla pasajera: hasta 3 reintentos (4.5),
// a 1, 2 y 4 segundos (4.24).
var EsperasPorDefecto = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// esperaDeReconexion es cuánto se espera para volver a conectarse si se cae RabbitMQ.
const esperaDeReconexion = 5 * time.Second

// Procesador aplica un mensaje. Si devuelve aplicacion.ErrInaplicable, el mensaje se confirma igual;
// cualquier otro error se reintenta.
type Procesador func(ctx context.Context, m aplicacion.Mensaje) error

// Consumidor lee la cola de Pistas.
type Consumidor struct {
	url        string
	procesador Procesador
	esperas    []time.Duration
}

// NuevoConsumidor arma el consumidor de la cola de Pistas.
func NuevoConsumidor(url string, procesador Procesador) *Consumidor {
	return &Consumidor{url: url, procesador: procesador, esperas: EsperasPorDefecto}
}

// Correr consume hasta que se cancele ctx. Si la conexión se cae, se vuelve a conectar.
func (c *Consumidor) Correr(ctx context.Context) {
	for {
		err := c.consumir(ctx)
		if ctx.Err() != nil {
			return
		}
		slog.ErrorContext(ctx, "se cortó el consumo de eventos; se vuelve a conectar", "queue", Cola, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(esperaDeReconexion):
		}
	}
}

func (c *Consumidor) consumir(ctx context.Context) error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("conectar con RabbitMQ: %w", err)
	}
	defer conn.Close()
	canal, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("abrir un canal: %w", err)
	}
	// De a un mensaje: el siguiente llega recién cuando se confirma el anterior.
	if err := canal.Qos(1, 0, false); err != nil {
		return fmt.Errorf("configurar el canal: %w", err)
	}
	entregas, err := canal.ConsumeWithContext(ctx, Cola, "pistas", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consumir %s: %w", Cola, err)
	}
	slog.InfoContext(ctx, "consumiendo eventos", "queue", Cola)
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, abierta := <-entregas:
			if !abierta {
				return errors.New("RabbitMQ cerró la entrega de mensajes")
			}
			c.procesar(ctx, e)
		}
	}
}

// procesar aplica un mensaje y lo confirma, o lo manda a fallidos si después de los reintentos
// sigue fallando (4.5, 4.20).
func (c *Consumidor) procesar(ctx context.Context, e amqp.Delivery) {
	ctx, span := observability.StartConsume(ctx, e.Headers, e.RoutingKey)
	m, err := c.aplicar(ctx, e)
	observability.EndSpan(span, err)
	// Los identificadores salen del cuerpo (4.10); si no se pudo leer, de las propiedades.
	eventoID, correlacionID := m.ID, m.CorrelationID
	if eventoID == "" {
		eventoID, correlacionID = e.MessageId, e.CorrelationId
	}

	switch {
	case err == nil:
		_ = e.Ack(false)
	case ctx.Err() != nil:
		// Pistas se está apagando: el mensaje vuelve a la cola.
		_ = e.Nack(false, true)
	case errors.Is(err, aplicacion.ErrInaplicable):
		// Reintentar no lo arregla: se confirma y queda el aviso (4.22).
		slog.WarnContext(ctx, "evento descartado", "event", e.RoutingKey, "event_id", eventoID,
			"correlation_id", correlacionID, "error", err)
		_ = e.Ack(false)
	default:
		// Sin volver a encolar: RabbitMQ lo pasa solo a pistas.eventos.fallidos (4.20). Este log es el
		// que dispara la alerta de mensajes fallidos (11.4).
		slog.ErrorContext(ctx, "evento enviado a fallidos", "event", e.RoutingKey, "event_id", eventoID,
			"correlation_id", correlacionID, "error", err)
		_ = e.Nack(false, false)
	}
}

// aplicar decodifica el mensaje y lo aplica, reintentando las fallas pasajeras.
func (c *Consumidor) aplicar(ctx context.Context, e amqp.Delivery) (aplicacion.Mensaje, error) {
	var m aplicacion.Mensaje
	if err := json.Unmarshal(e.Body, &m); err != nil || m.ID == "" {
		return m, fmt.Errorf("%w: mensaje inválido: %v", aplicacion.ErrInaplicable, err)
	}
	for intento := 0; ; intento++ {
		err := c.procesador(ctx, m)
		if err == nil || errors.Is(err, aplicacion.ErrInaplicable) || intento == len(c.esperas) {
			return m, err
		}
		slog.WarnContext(ctx, "reintento de un evento", "event", m.Type, "event_id", m.ID, "attempt", intento+1, "error", err)
		select {
		case <-ctx.Done():
			return m, ctx.Err()
		case <-time.After(c.esperas[intento]):
		}
	}
}
