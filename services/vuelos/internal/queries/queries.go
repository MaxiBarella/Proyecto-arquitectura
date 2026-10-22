package queries

import (
	"context"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// FlightReader es lo que necesitan las consultas de la base de vuelos.
type FlightReader interface {
	Get(ctx context.Context, id string) (*domain.Flight, error)
	ListByAirline(ctx context.Context, airline string, status domain.Status, page int) ([]domain.Flight, int64, error)
}

// Service es el lado de lectura de Vuelos (CQRS, 7.3). Los vuelos propios de una
// aerolínea se leen de MySQL, al instante. El tablero público va a leer del
// índice de búsqueda (14), en noviembre.
type Service struct {
	flights FlightReader
}

func New(flights FlightReader) *Service {
	return &Service{flights: flights}
}

// Own devuelve uno de los vuelos de la aerolínea del pedido (RN-30).
func (s *Service) Own(ctx context.Context, who commands.Identity, id string) (*domain.Flight, error) {
	f, err := s.flights.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := f.CheckOwner(who.Airline); err != nil {
		return nil, err
	}
	return f, nil
}

// ListOwn devuelve una página de los vuelos de la aerolínea del pedido (F-07).
func (s *Service) ListOwn(ctx context.Context, who commands.Identity, status domain.Status, page int) ([]domain.Flight, int64, error) {
	return s.flights.ListByAirline(ctx, who.Airline, status, page)
}
