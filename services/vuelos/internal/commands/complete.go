package commands

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// completeBatch es cuántos vuelos se finalizan por vuelta como máximo. Los que quedan
// se toman en la próxima, un minuto después (28.1).
const completeBatch = 100

// DueLister busca los vuelos programados cuyo bloque ya terminó.
type DueLister interface {
	DueForCompletion(ctx context.Context, now time.Time, limit int) ([]string, error)
}

// CompleteDue da por finalizados los vuelos programados cuyo bloque terminó (RN-32,
// F-22), cada uno con su "vuelo actualizado" para el tablero (14.5). Corre en las dos
// instancias de Vuelos: si la otra ya finalizó un vuelo, se saltea. Devuelve cuántos
// finalizó.
func (s *Service) CompleteDue(ctx context.Context, due DueLister) (int, error) {
	now := s.now()
	ids, err := due.DueForCompletion(ctx, now, completeBatch)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, id := range ids {
		_, err := s.flights.Update(ctx, id, func(f *domain.Flight) ([]domain.Event, error) {
			if err := f.Complete(now); err != nil {
				return nil, err
			}
			return []domain.Event{s.updated(ctx, f)}, nil
		})
		switch {
		case err == nil:
			done++
		case errors.Is(err, domain.ErrInvalidTransition), errors.Is(err, domain.ErrNotFound):
			// Lo finalizó la otra instancia, o cambió entre la búsqueda y ahora.
		default:
			return done, err
		}
	}
	if done > 0 {
		slog.InfoContext(ctx, "vuelos finalizados", "count", done)
	}
	return done, nil
}
