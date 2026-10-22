package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// Claves de ruteo de los eventos de Pistas que consume Vuelos (4.14).
const (
	EventSlotAssigned   = "pistas.slot.asignado"
	EventSlotCeded      = "pistas.slot.cedido"
	EventSlotReleased   = "pistas.slot.liberado_por_cierre"
	EventRunwayClosed   = "pistas.pista.cerrada"
	EventRunwayReopened = "pistas.pista.reabierta"
)

// ErrUnprocessable es un evento que no se puede aplicar y que reintentar no
// arregla: el vuelo no existe, ya está en un estado que no lo admite (por
// ejemplo, "slot asignado" para un vuelo cancelado) o los datos están mal. El
// consumidor lo confirma y deja un aviso en el log, sin mandarlo a fallidos (4.22).
var ErrUnprocessable = errors.New("el evento no se puede aplicar")

// Message es un mensaje recibido de RabbitMQ, con los campos de la 4.10.
type Message struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CorrelationID string          `json:"correlation_id"`
	DataVersion   int64           `json:"data_version"`
	Data          json.RawMessage `json:"data"`
}

// slotEventData son los datos de los eventos de slot de Pistas (4.12).
type slotEventData struct {
	FlightID   string    `json:"flight_id"`
	RunwayID   string    `json:"runway_id"`
	BlockStart time.Time `json:"block_start"`
	DivertedTo *string   `json:"diverted_to"`
}

// MessageStore es lo que necesita el consumidor de la base de vuelos.
type MessageStore interface {
	ApplyMessage(ctx context.Context, messageID, flightID string, change domain.Change) (bool, error)
}

// HandleEvent aplica un evento de Pistas al vuelo que corresponde, una sola vez
// por mensaje (4.9). Devuelve ErrUnprocessable si el evento no se puede aplicar;
// cualquier otro error es pasajero y el consumidor reintenta.
func (s *Service) HandleEvent(ctx context.Context, store MessageStore, m Message) error {
	var change func(f *domain.Flight) error
	switch m.Type {
	case EventSlotAssigned:
		// El vuelo pasa a programado con su slot (RN-08, RN-19).
		change = func(f *domain.Flight) error { return f.AssignSlot(slotOf(m), s.now()) }
	case EventSlotCeded:
		// El despegue pierde su slot ante un arribo y queda demorado (RN-12).
		change = func(f *domain.Flight) error { return f.LoseSlotToArrival(s.now()) }
	case EventSlotReleased:
		// Cierre de pista: el despegue queda demorado y el arribo desviado (RN-17, RN-18).
		change = func(f *domain.Flight) error {
			alternate := ""
			if d := dataOf(m); d.DivertedTo != nil {
				alternate = *d.DivertedTo
			}
			return f.ReleaseByClosure(alternate, s.now())
		}
	case EventRunwayClosed, EventRunwayReopened:
		// Vuelos los va a usar para el tablero, en noviembre (4.3). Por ahora no
		// cambian ningún vuelo.
		slog.InfoContext(ctx, "evento de pista sin efecto en los vuelos", "event", m.Type, "event_id", m.ID)
		return nil
	default:
		// Un evento nuevo de Pistas que Vuelos todavía no conoce: se confirma, así
		// no termina en fallidos (4.22).
		slog.WarnContext(ctx, "evento desconocido", "event", m.Type, "event_id", m.ID)
		return nil
	}

	d := dataOf(m)
	if d.FlightID == "" {
		return fmt.Errorf("%w: %s sin flight_id", ErrUnprocessable, m.Type)
	}
	applied, err := store.ApplyMessage(ctx, m.ID, d.FlightID, func(f *domain.Flight) ([]domain.Event, error) {
		return nil, change(f)
	})
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrInvalidTransition), errors.Is(err, domain.ErrInvalidFlight):
		return fmt.Errorf("%w: %v", ErrUnprocessable, err)
	case err != nil:
		return err
	case !applied:
		slog.InfoContext(ctx, "evento repetido, ya aplicado", "event", m.Type, "event_id", m.ID)
	}
	return nil
}

func dataOf(m Message) slotEventData {
	var d slotEventData
	_ = json.Unmarshal(m.Data, &d)
	return d
}

func slotOf(m Message) domain.Slot {
	d := dataOf(m)
	return domain.Slot{RunwayID: d.RunwayID, BlockStart: d.BlockStart.UTC()}
}
