package aplicacion

import (
	"context"
	"log/slog"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// IntervaloDelRelay es cada cuánto se revisa el outbox (4.5).
const IntervaloDelRelay = time.Second

// loteDelRelay es cuántos eventos se publican por vuelta como máximo.
const loteDelRelay = 100

// Relay publica los eventos del outbox. Entrega cada evento al menos una vez: si se cae después
// de publicar y antes de marcarlo, lo publica de nuevo, y el consumidor lo descarta por su id (4.9).
type Relay struct {
	outbox     ports.Outbox
	publicador ports.Publicador
	log        *slog.Logger
}

// NuevoRelay arma el relay.
func NuevoRelay(o ports.Outbox, p ports.Publicador, log *slog.Logger) *Relay {
	return &Relay{outbox: o, publicador: p, log: log}
}

// Correr revisa el outbox cada IntervaloDelRelay hasta que se cancele el contexto.
func (r *Relay) Correr(ctx context.Context) {
	t := time.NewTicker(IntervaloDelRelay)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.UnaVuelta(ctx)
		}
	}
}

// UnaVuelta publica, en orden, lo pendiente. Si un evento falla, corta para no desordenar y lo
// reintenta en la vuelta siguiente.
func (r *Relay) UnaVuelta(ctx context.Context) {
	pendientes, err := r.outbox.Pendientes(ctx, loteDelRelay)
	if err != nil {
		r.log.ErrorContext(ctx, "no se pudo leer el outbox", "error", err)
		return
	}
	for _, e := range pendientes {
		if err := r.publicador.Publicar(ctx, e); err != nil {
			r.log.ErrorContext(ctx, "no se pudo publicar el evento; se reintenta en la próxima vuelta",
				"error", err, "evento_id", e.ID, "tipo", e.Tipo, "correlacion_id", e.CorrelacionID)
			return
		}
		if err := r.outbox.MarcarPublicado(ctx, e.Secuencia); err != nil {
			r.log.ErrorContext(ctx, "evento publicado pero no marcado; se va a publicar de nuevo",
				"error", err, "evento_id", e.ID)
			return
		}
		r.log.InfoContext(ctx, "evento publicado", "evento_id", e.ID, "tipo", e.Tipo, "correlacion_id", e.CorrelacionID)
	}
}
