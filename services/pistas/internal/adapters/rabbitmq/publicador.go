package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// Exchange es el único exchange del sistema, de tipo topic (4.10).
const Exchange = "aeropuerto.eventos"

// versionDelFormato es la versión del formato de los eventos de Pistas (4.10).
const versionDelFormato = 1

// plazoDePublicacion es cuánto se espera la confirmación de RabbitMQ antes de darlo por fallido.
const plazoDePublicacion = 5 * time.Second

// ErrSinCola indica que RabbitMQ devolvió el mensaje porque ninguna cola lo recibe. El evento queda
// en el outbox hasta que exista una (4.13).
var ErrSinCola = errors.New("ninguna cola recibe el evento")

// Publicador publica eventos en RabbitMQ con confirmación del broker. Si la conexión se cae, la
// vuelve a abrir en el próximo intento: el relay reintenta cada segundo, así que no hace falta más.
type Publicador struct {
	url string

	mu    sync.Mutex
	conn  *amqp.Connection
	canal *amqp.Channel
	// devueltos recibe los mensajes que RabbitMQ devuelve porque no los toma ninguna cola.
	devueltos chan amqp.Return
}

var _ ports.Publicador = (*Publicador)(nil)

// NuevoPublicador no se conecta todavía: lo hace en el primer Publicar. Así el servicio arranca y
// atiende pedidos aunque RabbitMQ no esté listo; los eventos esperan en el outbox.
func NuevoPublicador(url string) *Publicador {
	return &Publicador{url: url}
}

// mensaje es el sobre de la decisión 4.10.
type mensaje struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CorrelationID string          `json:"correlation_id"`
	DataVersion   int64           `json:"data_version"`
	Data          json.RawMessage `json:"data"`
}

// Codificar arma el cuerpo JSON del mensaje. Es el contrato con Vuelos.
func Codificar(e ports.EventoGuardado) ([]byte, error) {
	return json.Marshal(mensaje{
		ID:            e.ID,
		Type:          e.Tipo,
		Version:       versionDelFormato,
		OccurredAt:    e.Fecha.UTC(),
		CorrelationID: e.CorrelacionID,
		DataVersion:   e.Secuencia,
		Data:          e.DatosJSON,
	})
}

// Publicar implementa ports.Publicador. Devuelve nil solo si RabbitMQ confirmó el mensaje y lo dejó
// en al menos una cola: recién ahí se puede marcar como publicado.
func (p *Publicador) Publicar(ctx context.Context, e ports.EventoGuardado) error {
	cuerpo, err := Codificar(e)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	canal, err := p.canalAbierto()
	if err != nil {
		return err
	}
	p.descartarDevueltosViejos()

	ctx, cancelar := context.WithTimeout(ctx, plazoDePublicacion)
	defer cancelar()
	// mandatory=true: si ninguna cola lo recibe, RabbitMQ lo devuelve en vez de descartarlo.
	confirmacion, err := canal.PublishWithDeferredConfirmWithContext(ctx, Exchange, e.Tipo, true, false, amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		MessageId:     e.ID,
		Type:          e.Tipo,
		CorrelationId: e.CorrelacionID,
		Timestamp:     e.Fecha,
		Body:          cuerpo,
	})
	if err != nil {
		p.cerrar()
		return fmt.Errorf("publicar en RabbitMQ: %w", err)
	}
	ok, err := confirmacion.WaitContext(ctx)
	if err != nil {
		p.cerrar()
		return fmt.Errorf("esperar la confirmación de RabbitMQ: %w", err)
	}
	if !ok {
		return errors.New("RabbitMQ rechazó el mensaje")
	}
	// RabbitMQ manda la devolución antes que la confirmación, así que si hubo una, ya llegó.
	select {
	case d := <-p.devueltos:
		if d.MessageId == e.ID {
			return fmt.Errorf("%w (%s)", ErrSinCola, e.Tipo)
		}
	default:
	}
	return nil
}

// descartarDevueltosViejos vacía devoluciones que hayan quedado de publicaciones anteriores.
func (p *Publicador) descartarDevueltosViejos() {
	for {
		select {
		case _, abierto := <-p.devueltos:
			if !abierto {
				return
			}
		default:
			return
		}
	}
}

// canalAbierto devuelve el canal, abriéndolo si hace falta. Declara el exchange cada vez que
// conecta: declararlo de nuevo con la misma configuración no hace nada.
func (p *Publicador) canalAbierto() (*amqp.Channel, error) {
	if p.canal != nil && !p.canal.IsClosed() {
		return p.canal, nil
	}
	p.cerrar()
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return nil, fmt.Errorf("conectar con RabbitMQ: %w", err)
	}
	canal, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("abrir un canal de RabbitMQ: %w", err)
	}
	if err := canal.ExchangeDeclare(Exchange, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("declarar el exchange: %w", err)
	}
	if err := canal.Confirm(false); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("activar las confirmaciones: %w", err)
	}
	p.devueltos = canal.NotifyReturn(make(chan amqp.Return, 16))
	p.conn, p.canal = conn, canal
	return canal, nil
}

func (p *Publicador) cerrar() {
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.conn, p.canal, p.devueltos = nil, nil, nil
}

// Cerrar libera la conexión al apagar el servicio.
func (p *Publicador) Cerrar() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cerrar()
}
