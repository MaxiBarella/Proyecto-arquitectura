package queries

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
)

// IndexedFlight es un vuelo como se guarda en el índice del tablero (ADR-006): los
// datos de "vuelo actualizado", el horario que vale y la versión del dato.
type IndexedFlight struct {
	commands.FlightUpdatedData
	// BoardAt es el horario del tablero: el del bloque si el vuelo tiene slot, y el
	// publicado si no (RN-39). Con él se ordena y se filtra por fecha.
	BoardAt time.Time
	// DataVersion es la versión del vuelo. Solr descarta una escritura con una
	// versión menor o igual a la que ya tiene (14.6).
	DataVersion int64
}

// FlightIndex es el índice de búsqueda del tablero.
type FlightIndex interface {
	Upsert(ctx context.Context, f IndexedFlight) error
}

// Indexer escribe en el índice cada "vuelo actualizado" de la cola vuelos.indice (14.5).
type Indexer struct {
	index FlightIndex
}

func NewIndexer(index FlightIndex) *Indexer {
	return &Indexer{index: index}
}

// HandleEvent escribe el vuelo del evento en el índice. Devuelve
// commands.ErrUnprocessable si el mensaje no es un "vuelo actualizado" válido, y
// cualquier otro error si falló el índice, que el consumidor reintenta (14.10).
// Repetirlo no hace daño: Solr descarta una versión que ya tiene (14.6).
func (ix *Indexer) HandleEvent(ctx context.Context, m commands.Message) error {
	if m.Type != commands.EventFlightUpdated {
		return fmt.Errorf("%w: la cola del índice no espera %s", commands.ErrUnprocessable, m.Type)
	}
	var d commands.FlightUpdatedData
	if err := json.Unmarshal(m.Data, &d); err != nil {
		return fmt.Errorf("%w: datos inválidos: %v", commands.ErrUnprocessable, err)
	}
	if d.ID == "" || m.DataVersion < 1 {
		return fmt.Errorf("%w: falta el id del vuelo o la versión del dato", commands.ErrUnprocessable)
	}
	boardAt := d.ScheduledAt
	if d.Slot != nil {
		boardAt = d.Slot.BlockStart
	}
	return ix.index.Upsert(ctx, IndexedFlight{FlightUpdatedData: d, BoardAt: boardAt.UTC(), DataVersion: m.DataVersion})
}
