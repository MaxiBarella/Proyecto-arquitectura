package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// Exchange es el único exchange del sistema, de tipo topic (4.8, docs/contracts/pistas-eventos.md).
const Exchange = "aeropuerto"

// versionDelFormato es la versión de "datos" de todos los eventos de Pistas.
const versionDelFormato = 1

// Publicador publica eventos en RabbitMQ con confirmación del broker. Si la conexión se cae, la
// vuelve a abrir en el próximo intento: el relay reintenta cada segundo, así que no hace falta más.
type Publicador struct {
	url string

	mu    sync.Mutex
	conn  *amqp.Connection
	canal *amqp.Channel
}

var _ ports.Publicador = (*Publicador)(nil)

// NuevoPublicador no se conecta todavía: lo hace en el primer Publicar. Así el servicio arranca y
// atiende pedidos aunque RabbitMQ no esté listo; los eventos esperan en el outbox.
func NuevoPublicador(url string) *Publicador {
	return &Publicador{url: url}
}

// mensaje es el sobre de la decisión 4.8.
type mensaje struct {
	ID            string          `json:"id"`
	Tipo          string          `json:"tipo"`
	Version       int             `json:"version"`
	Fecha         time.Time       `json:"fecha"`
	CorrelacionID string          `json:"correlacion_id"`
	VersionDato   int64           `json:"version_dato"`
	Datos         json.RawMessage `json:"datos"`
}

// Publicar implementa ports.Publicador. Vuelve recién cuando el broker confirmó el mensaje.
func (p *Publicador) Publicar(ctx context.Context, e ports.EventoGuardado) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	canal, err := p.canalAbierto()
	if err != nil {
		return err
	}
	cuerpo, err := json.Marshal(mensaje{
		ID:            e.ID,
		Tipo:          e.Tipo,
		Version:       versionDelFormato,
		Fecha:         e.Fecha.In(domain.ZonaCordoba),
		CorrelacionID: e.CorrelacionID,
		VersionDato:   e.Secuencia,
		Datos:         e.DatosJSON,
	})
	if err != nil {
		return err
	}
	confirmacion, err := canal.PublishWithDeferredConfirmWithContext(ctx, Exchange, e.Tipo, false, false, amqp.Publishing{
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
		return err
	}
	ok, err := confirmacion.WaitContext(ctx)
	if err != nil {
		p.cerrar()
		return err
	}
	if !ok {
		return errors.New("el broker rechazó el mensaje")
	}
	return nil
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
		return nil, fmt.Errorf("no se pudo conectar a RabbitMQ: %w", err)
	}
	canal, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := canal.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, err
	}
	if err := canal.Confirm(false); err != nil {
		conn.Close()
		return nil, err
	}
	p.conn, p.canal = conn, canal
	return canal, nil
}

func (p *Publicador) cerrar() {
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.conn, p.canal = nil, nil
}

// Cerrar libera la conexión al apagar el servicio.
func (p *Publicador) Cerrar() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cerrar()
}
