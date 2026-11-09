package aplicacion

import (
	"context"
	"errors"
	"log/slog"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// Los casos de uso del operador (F-09, F-10, F-11, F-20, F-21), detrás de las rutas /runways y
// /alternate-airports del contrato (23.2, 23.3).

// ListarPistas devuelve las pistas y lo último que Pistas sabe del clima, para la pantalla del
// operador y el aviso de "Clima sin datos" (F-21).
func (a *Pistas) ListarPistas(ctx context.Context) ([]domain.Pista, domain.EstadoClima, error) {
	pistas, err := a.repo.Pistas(ctx)
	if err != nil {
		return nil, domain.EstadoClima{}, err
	}
	clima, err := a.repo.ClimaActual(ctx)
	return pistas, clima, err
}

// AltaDePista da de alta una pista (F-09). Nace abierta, salvo que el último dato del clima diga
// que no se puede operar: entonces nace cerrada por el clima, como las demás (RN-15, 23.5).
func (a *Pistas) AltaDePista(ctx context.Context, id string) (domain.Pista, error) {
	if err := domain.ValidarNombreDePista(id); err != nil {
		return domain.Pista{}, err
	}
	var nueva domain.Pista
	err := a.conReintentos(ctx, func(ctx context.Context, tx ports.Transaccion) error {
		clima, err := tx.Clima(ctx)
		if err != nil {
			return err
		}
		nueva = domain.Pista{ID: id, Abierta: true}
		if clima.Operable != nil && !*clima.Operable {
			nueva = domain.Pista{ID: id, Abierta: false, CerradaPor: domain.CerradaPorClima}
		}
		if err := tx.CrearPista(ctx, nueva); err != nil {
			return err
		}
		if !nueva.Abierta {
			return tx.GuardarEvento(ctx, a.evento(ctx, domain.EventoPistaCerrada,
				domain.DatosPista{PistaID: id, Motivo: domain.CerradaPorClima}))
		}
		return nil
	})
	if err == nil {
		slog.InfoContext(ctx, "pista dada de alta", "runway_id", id, "abierta", nueva.Abierta)
	}
	return nueva, err
}

// CerrarPista cierra una pista a mano (F-10, RN-16). Usa el mismo cierre que el clima: libera los
// slots de ahí en adelante y guarda los eventos en la misma transacción (ADR-004). Si la había
// cerrado el clima, pasa a cerrada por el operador, así no se reabre sola (23.5). Si ya la había
// cerrado el operador, no cambia nada.
func (a *Pistas) CerrarPista(ctx context.Context, id string) (domain.Pista, error) {
	var pista domain.Pista
	err := a.conReintentos(ctx, func(ctx context.Context, tx ports.Transaccion) error {
		p, err := pistaConBloqueo(ctx, tx, id)
		if err != nil {
			return err
		}
		if !p.Abierta && p.CerradaPor == domain.CerradaPorOperador {
			pista = p
			return nil
		}
		alternativos, err := tx.Alternativos(ctx)
		if err != nil {
			return err
		}
		if err := a.cerrar(ctx, tx, p, domain.CerradaPorOperador, alternativos); err != nil {
			return err
		}
		pista = domain.Pista{ID: p.ID, Abierta: false, CerradaPor: domain.CerradaPorOperador}
		return nil
	})
	return pista, err
}

// ReabrirPista reabre una pista (F-11). Una cerrada por el clima solo se reabre a mano cuando
// Clima no tiene datos; una condición forzada cuenta como dato (RN-42, 23.4). Si ya estaba
// abierta, no cambia nada.
func (a *Pistas) ReabrirPista(ctx context.Context, id string) (domain.Pista, error) {
	var pista domain.Pista
	err := a.conReintentos(ctx, func(ctx context.Context, tx ports.Transaccion) error {
		p, err := pistaConBloqueo(ctx, tx, id)
		if err != nil {
			return err
		}
		if p.Abierta {
			pista = p
			return nil
		}
		if p.CerradaPor == domain.CerradaPorClima {
			clima, err := tx.Clima(ctx)
			if err != nil {
				return err
			}
			if clima.ConDatos {
				return domain.ErrCerradaPorClima
			}
		}
		motivo := p.CerradaPor
		pista = domain.Pista{ID: p.ID, Abierta: true}
		if err := tx.GuardarPista(ctx, pista); err != nil {
			return err
		}
		if err := tx.GuardarEvento(ctx, a.evento(ctx, domain.EventoPistaReabierta,
			domain.DatosPista{PistaID: p.ID, Motivo: motivo})); err != nil {
			return err
		}
		slog.InfoContext(ctx, "pista reabierta a mano", "runway_id", p.ID, "cerrada_por", motivo)
		return nil
	})
	return pista, err
}

// ListarAlternativos devuelve la lista de alternativos, del más cercano al más lejano (RN-25).
func (a *Pistas) ListarAlternativos(ctx context.Context) ([]domain.Alternativo, error) {
	return a.repo.Alternativos(ctx)
}

// CambiarDisponibilidad marca un alternativo como disponible o no (F-20, RN-26). Vale para los
// arribos que se desvíen de ahí en adelante.
func (a *Pistas) CambiarDisponibilidad(ctx context.Context, icao string, disponible bool) (domain.Alternativo, error) {
	var alt domain.Alternativo
	err := a.conReintentos(ctx, func(ctx context.Context, tx ports.Transaccion) error {
		var err error
		alt, err = tx.GuardarAlternativo(ctx, icao, disponible)
		return err
	})
	if err == nil {
		slog.InfoContext(ctx, "alternativo actualizado", "icao", icao, "disponible", disponible)
	}
	return alt, err
}

// pistaConBloqueo busca una pista entre las bloqueadas, así no se cruza con un pedido de slot ni
// con un evento de clima (ADR-004).
func pistaConBloqueo(ctx context.Context, tx ports.Transaccion, id string) (domain.Pista, error) {
	pistas, err := tx.PistasConBloqueo(ctx)
	if err != nil {
		return domain.Pista{}, err
	}
	for _, p := range pistas {
		if p.ID == id {
			return p, nil
		}
	}
	return domain.Pista{}, domain.ErrPistaInexistente
}

// conReintentos corre fn en una transacción y la repite si choca con otra simultánea, como la
// asignación de slots (ADR-004).
func (a *Pistas) conReintentos(ctx context.Context, fn func(context.Context, ports.Transaccion) error) error {
	var err error
	for intento := 1; intento <= intentosPorConflicto; intento++ {
		err = a.repo.EnTransaccion(ctx, fn)
		if !errors.Is(err, ports.ErrConflicto) {
			return err
		}
	}
	return err
}
