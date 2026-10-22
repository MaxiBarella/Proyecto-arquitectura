package aplicacion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// Claves de ruteo de los eventos que recibe Pistas en su cola pistas.eventos (4.14, 4.19, 9.14).
const (
	EventoVueloCancelado = "vuelos.vuelo.cancelado"
	prefijoClima         = "clima."
)

// ErrInaplicable es un evento que no se puede aplicar y que reintentar no arregla: le faltan datos o
// no se puede leer. El consumidor lo confirma y deja un aviso en el log, sin mandarlo a fallidos (4.22).
var ErrInaplicable = errors.New("el evento no se puede aplicar")

// Mensaje es un mensaje recibido de RabbitMQ, con los campos de la 4.10.
type Mensaje struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CorrelationID string          `json:"correlation_id"`
	DataVersion   int64           `json:"data_version"`
	Data          json.RawMessage `json:"data"`
}

// ProcesarEvento aplica un evento recibido, una sola vez por mensaje (4.9). Devuelve ErrInaplicable
// si el evento no se puede aplicar; cualquier otro error es pasajero y el consumidor reintenta.
func (a *Pistas) ProcesarEvento(ctx context.Context, m Mensaje) error {
	switch {
	case m.Type == EventoVueloCancelado:
		return a.liberarSlotDeVueloCancelado(ctx, m)
	case strings.HasPrefix(m.Type, prefijoClima):
		// Cerrar y reabrir pistas por clima se programa en noviembre (8.1). Hasta entonces el evento
		// se confirma para que no se acumule en la cola.
		slog.InfoContext(ctx, "evento de clima sin efecto hasta noviembre", "event", m.Type, "event_id", m.ID)
		return nil
	default:
		// Un evento que Pistas todavía no conoce: se confirma, así no termina en fallidos (4.22).
		slog.WarnContext(ctx, "evento desconocido", "event", m.Type, "event_id", m.ID)
		return nil
	}
}

// liberarSlotDeVueloCancelado deja libre el slot de un vuelo cancelado (RN-23, CA-17). Si el vuelo
// no tenía slot, no hay nada que liberar.
func (a *Pistas) liberarSlotDeVueloCancelado(ctx context.Context, m Mensaje) error {
	var datos struct {
		FlightID string `json:"flight_id"`
	}
	if err := json.Unmarshal(m.Data, &datos); err != nil || datos.FlightID == "" {
		return fmt.Errorf("%w: %s sin flight_id", ErrInaplicable, m.Type)
	}
	return a.repo.EnTransaccion(ctx, func(ctx context.Context, tx ports.Transaccion) error {
		// El id del mensaje se guarda en la misma transacción que el efecto: si llega dos veces,
		// la segunda no hace nada (4.9).
		nuevo, err := tx.MarcarProcesado(ctx, m.ID)
		if err != nil || !nuevo {
			if err == nil {
				slog.InfoContext(ctx, "evento repetido, ya aplicado", "event", m.Type, "event_id", m.ID)
			}
			return err
		}
		slot, err := tx.SlotDelVuelo(ctx, datos.FlightID)
		if err != nil || slot == nil {
			return err
		}
		if err := tx.Liberar(ctx, slot.ID); err != nil {
			return err
		}
		slog.InfoContext(ctx, "slot liberado por cancelación", "flight_id", datos.FlightID,
			"runway_id", slot.PistaID, "block_start", slot.InicioBloque)
		return nil
	})
}
