package commands

import (
	"context"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/observability"
)

// Claves de ruteo de los eventos que publica Vuelos (4.14).
const EventFlightCancelled = "vuelos.vuelo.cancelado"

// RoleAirline es el valor de X-User-Role de una aerolínea (15.9).
const RoleAirline = "airline"

// Identity es quién hace el pedido, según los encabezados que pone el gateway (15.6).
type Identity struct {
	UserID  string
	Role    string
	Airline string
}

// FlightStore es lo que necesitan los comandos de la base de vuelos.
type FlightStore interface {
	Create(ctx context.Context, f *domain.Flight, events ...domain.Event) error
	Get(ctx context.Context, id string) (*domain.Flight, error)
	Update(ctx context.Context, id string, change domain.Change) (*domain.Flight, error)
}

// Service es el lado de escritura de Vuelos (CQRS, 7.3).
type Service struct {
	flights FlightStore
	slots   SlotRequester
	now     func() time.Time
}

func New(flights FlightStore, slots SlotRequester) *Service {
	return &Service{flights: flights, slots: slots, now: func() time.Time { return time.Now().UTC() }}
}

// Publish publica un vuelo de la aerolínea del pedido (F-02). Queda en
// "pendiente de slot" (RN-06).
func (s *Service) Publish(ctx context.Context, who Identity, d domain.NewFlightData) (*domain.Flight, error) {
	f, err := domain.NewFlight(domain.NewID(), who.Airline, d, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.flights.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// Cancel cancela uno de los vuelos de la aerolínea (F-06, RN-31) y deja en el
// outbox "vuelo cancelado", con el que Pistas libera el slot (RN-23).
func (s *Service) Cancel(ctx context.Context, who Identity, id string) (*domain.Flight, error) {
	return s.flights.Update(ctx, id, func(f *domain.Flight) ([]domain.Event, error) {
		if err := f.Cancel(who.Airline, s.now()); err != nil {
			return nil, err
		}
		// Datos del evento: 4.12.
		return []domain.Event{s.event(ctx, EventFlightCancelled, map[string]string{"flight_id": f.ID})}, nil
	})
}

func (s *Service) event(ctx context.Context, eventType string, data any) domain.Event {
	return domain.Event{
		ID:            domain.NewID(),
		Type:          eventType,
		OccurredAt:    s.now(),
		CorrelationID: correlationID(ctx),
		TraceParent:   observability.TraceParent(ctx),
		Data:          data,
	}
}

// correlationID es el identificador de la traza en curso (11.2), o uno nuevo si
// no hay traza.
func correlationID(ctx context.Context) string {
	if id := observability.TraceID(ctx); id != "" {
		return id
	}
	return domain.NewID()
}
