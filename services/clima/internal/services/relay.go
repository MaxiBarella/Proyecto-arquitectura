package services

import (
	"context"
	"log/slog"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

// EventPublisher publica un evento y devuelve nil solo si RabbitMQ lo confirmó.
type EventPublisher interface {
	Publish(ctx context.Context, e models.PendingEvent) error
}

// OutboxStore es lo que necesita el relay del documento de aptitud.
type OutboxStore interface {
	PendingEvents(ctx context.Context) ([]models.AptitudeState, error)
	MarkPublished(ctx context.Context, airport, eventID string) error
}

// Relay publica los eventos que quedaron en el outbox (4.4, 4.7).
type Relay struct {
	outbox    OutboxStore
	publisher EventPublisher
}

func NewRelay(outbox OutboxStore, publisher EventPublisher) *Relay {
	return &Relay{outbox: outbox, publisher: publisher}
}

// PublishPending publica los eventos pendientes en orden y los saca del outbox.
// Si uno falla, se detiene ahí, para no publicar uno posterior antes que él:
// se reintenta en la próxima vuelta. Si el proceso se cae entre publicar y sacar
// el evento, se vuelve a publicar; los consumidores lo descartan por su id (4.9).
func (r *Relay) PublishPending(ctx context.Context) error {
	states, err := r.outbox.PendingEvents(ctx)
	if err != nil {
		return err
	}
	for _, st := range states {
		for _, e := range st.Outbox {
			if err := r.publisher.Publish(ctx, e); err != nil {
				return err
			}
			if err := r.outbox.MarkPublished(ctx, st.Airport, e.ID); err != nil {
				return err
			}
			slog.InfoContext(ctx, "evento publicado", "event", e.Type, "event_id", e.ID,
				"correlation_id", e.CorrelationID, "data_version", e.DataVersion)
		}
	}
	return nil
}
