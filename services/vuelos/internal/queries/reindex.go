package queries

import (
	"context"
	"fmt"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// reindexBatch es cuántos vuelos se leen de MySQL por tanda al reconstruir el índice.
const reindexBatch = 200

// AllFlightsReader recorre todos los vuelos de MySQL, de a tandas ordenadas por id.
type AllFlightsReader interface {
	ListAfter(ctx context.Context, afterID string, limit int) ([]domain.Flight, error)
}

// Reindex vuelve a cargar en el índice todos los vuelos de MySQL, que es la fuente de
// verdad (14.3, 14.11). Se usa cuando Solr perdió sus datos o quedó desfasado, por
// ejemplo después de que unos eventos fueron a vuelos.indice.fallidos (14.10). Cada
// vuelo va con su versión, así que si el consumidor escribe algo más nuevo al mismo
// tiempo, Solr se queda con lo más nuevo (14.6). Devuelve cuántos vuelos escribió.
func Reindex(ctx context.Context, flights AllFlightsReader, index FlightIndex) (int, error) {
	written, after := 0, ""
	for {
		batch, err := flights.ListAfter(ctx, after, reindexBatch)
		if err != nil {
			return written, err
		}
		for i := range batch {
			f := &batch[i]
			if err := index.Upsert(ctx, indexed(commands.NewFlightUpdatedData(f), f.Version)); err != nil {
				return written, fmt.Errorf("escribir el vuelo %s en el índice: %w", f.ID, err)
			}
			written++
		}
		if len(batch) < reindexBatch {
			return written, nil
		}
		after = batch[len(batch)-1].ID
	}
}

// indexed arma un vuelo del índice: con el horario del tablero, el del bloque si tiene
// slot y el publicado si no (RN-39), y su versión (14.6).
func indexed(d commands.FlightUpdatedData, version int64) IndexedFlight {
	boardAt := d.ScheduledAt
	if d.Slot != nil {
		boardAt = d.Slot.BlockStart
	}
	return IndexedFlight{FlightUpdatedData: d, BoardAt: boardAt.UTC(), DataVersion: version}
}
