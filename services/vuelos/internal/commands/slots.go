package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// Errores del pedido de slot. Los devuelve el cliente de Pistas y la capa HTTP
// los traduce a los códigos del contrato.
var (
	ErrRunwayClosed       = errors.New("la pista está cerrada")
	ErrRunwayNotFound     = errors.New("la pista no existe")
	ErrRunwaysUnavailable = errors.New("Pistas no está disponible")
)

// SlotTakenError es el rechazo por slot ocupado, con los libres más cercanos (RN-09).
type SlotTakenError struct {
	Nearest []domain.Slot
}

func (e *SlotTakenError) Error() string { return "el slot está ocupado" }

// SlotRequest es lo que Vuelos le manda a Pistas al pedir un slot (4.15).
type SlotRequest struct {
	FlightID   string
	FlightType domain.FlightType
	Slot       domain.Slot
}

// SlotRequester le pide un slot a Pistas. Devuelve el slot asignado, o uno de los
// errores de arriba.
type SlotRequester interface {
	RequestSlot(ctx context.Context, req SlotRequest) (domain.Slot, error)
}

// RequestSlot le pide a Pistas el slot que eligió la aerolínea (F-03, F-05). Si
// Pistas lo asigna, el vuelo no cambia acá: pasa a programado cuando llega
// pistas.slot.asignado (4.1). La llamada se hace sin tener el vuelo bloqueado,
// para no frenar otros cambios mientras se espera a Pistas.
func (s *Service) RequestSlot(ctx context.Context, who Identity, id string, slot domain.Slot) (*domain.Flight, domain.Slot, error) {
	if err := domain.ValidateSlot(slot); err != nil {
		return nil, domain.Slot{}, err
	}
	f, err := s.flights.Get(ctx, id)
	if err != nil {
		return nil, domain.Slot{}, err
	}
	if err := f.CheckOwner(who.Airline); err != nil {
		return nil, domain.Slot{}, err
	}
	if err := f.CanRequestSlot(); err != nil {
		return nil, domain.Slot{}, err
	}
	assigned, err := s.slots.RequestSlot(ctx, SlotRequest{FlightID: f.ID, FlightType: f.Type, Slot: slot})
	if err != nil {
		return nil, domain.Slot{}, fmt.Errorf("pedir el slot a Pistas: %w", err)
	}
	return f, assigned, nil
}
