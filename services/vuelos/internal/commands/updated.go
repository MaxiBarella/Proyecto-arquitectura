package commands

import (
	"context"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// EventFlightUpdated es el evento que deja Vuelos en el outbox con cada cambio de un
// vuelo, para mantener al día el índice de búsqueda del tablero (14.5). Lo lee Vuelos
// mismo, de la cola vuelos.indice. La versión del dato, que el índice usa para
// descartar un cambio viejo (14.6), va en el sobre del mensaje (data_version).
const EventFlightUpdated = "vuelos.vuelo.actualizado"

// FlightUpdatedData son los datos de "vuelo actualizado": los que muestra el tablero,
// con los mismos nombres que el contrato de Vuelos (Flight en openapi.yaml).
type FlightUpdatedData struct {
	ID           string          `json:"id"`
	FlightNumber string          `json:"flight_number"`
	Airline      string          `json:"airline"`
	Type         string          `json:"type"`
	Scope        string          `json:"scope"`
	Origin       string          `json:"origin"`
	Destination  string          `json:"destination"`
	ScheduledAt  time.Time       `json:"scheduled_at"`
	Status       string          `json:"status"`
	Slot         *FlightSlotData `json:"slot"`
	DivertedTo   *string         `json:"diverted_to"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// FlightSlotData es el slot de un vuelo dentro de "vuelo actualizado".
type FlightSlotData struct {
	RunwayID   string    `json:"runway_id"`
	BlockStart time.Time `json:"block_start"`
}

// NewFlightUpdatedData arma los datos del evento a partir del vuelo ya cambiado.
func NewFlightUpdatedData(f *domain.Flight) FlightUpdatedData {
	d := FlightUpdatedData{
		ID:           f.ID,
		FlightNumber: f.Number,
		Airline:      f.Airline,
		Type:         string(f.Type),
		Scope:        string(f.Scope),
		Origin:       f.Origin,
		Destination:  f.Destination,
		ScheduledAt:  f.ScheduledAt.UTC(),
		Status:       string(f.Status),
		CreatedAt:    f.CreatedAt.UTC(),
		UpdatedAt:    f.UpdatedAt.UTC(),
	}
	if f.Slot != nil {
		d.Slot = &FlightSlotData{RunwayID: f.Slot.RunwayID, BlockStart: f.Slot.BlockStart.UTC()}
	}
	if f.DivertedTo != "" {
		alt := f.DivertedTo
		d.DivertedTo = &alt
	}
	return d
}

// updated es el evento "vuelo actualizado" del vuelo f, ya cambiado.
func (s *Service) updated(ctx context.Context, f *domain.Flight) domain.Event {
	return s.event(ctx, EventFlightUpdated, NewFlightUpdatedData(f))
}
