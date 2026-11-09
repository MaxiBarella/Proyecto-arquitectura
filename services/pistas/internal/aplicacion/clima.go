package aplicacion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// Claves de ruteo de los eventos de Clima (9.14).
const (
	EventoAptitudCambio = "clima.aptitud.cambio"
	EventoClimaSinDatos = "clima.sin_datos"
)

// datosClima son los datos de los eventos de Clima que usa Pistas (9.14). "Sin datos" no trae
// operable.
type datosClima struct {
	Airport  string `json:"airport"`
	Operable *bool  `json:"operable"`
}

// aplicarClima atiende "aptitud cambió" y "Clima sin datos" (4.2, 6.19). Todo pasa en una sola
// transacción: el estado del clima, las pistas, los slots liberados y sus eventos (ADR-004).
func (a *Pistas) aplicarClima(ctx context.Context, m Mensaje) error {
	var datos datosClima
	if err := json.Unmarshal(m.Data, &datos); err != nil {
		return fmt.Errorf("%w: %s no se puede leer", ErrInaplicable, m.Type)
	}
	if datos.Airport != domain.NuestroAeropuerto {
		// Clima atiende solo nuestro aeropuerto (6.9); otro no tiene pistas acá.
		slog.WarnContext(ctx, "evento de clima de otro aeropuerto", "event", m.Type, "airport", datos.Airport)
		return nil
	}
	if m.Type == EventoAptitudCambio && datos.Operable == nil {
		return fmt.Errorf("%w: %s sin operable", ErrInaplicable, m.Type)
	}

	return a.repo.EnTransaccion(ctx, func(ctx context.Context, tx ports.Transaccion) error {
		nuevo, err := tx.MarcarProcesado(ctx, m.ID)
		if err != nil || !nuevo {
			if err == nil {
				slog.InfoContext(ctx, "evento repetido, ya aplicado", "event", m.Type, "event_id", m.ID)
			}
			return err
		}
		estado, err := tx.Clima(ctx)
		if err != nil {
			return err
		}

		if m.Type == EventoClimaSinDatos {
			// Las pistas quedan como están: el operador decide a mano (RN-24, CA-20).
			estado.ConDatos = false
			slog.InfoContext(ctx, "clima sin datos: las pistas no cambian solas")
			return tx.GuardarClima(ctx, estado)
		}

		// "Aptitud cambió" también llega cuando vuelven los datos después de un "sin datos" (9.14).
		estado.Operable, estado.ConDatos = datos.Operable, true
		if err := tx.GuardarClima(ctx, estado); err != nil {
			return err
		}
		if *datos.Operable {
			return a.reabrirPorClima(ctx, tx)
		}
		return a.cerrarPorClima(ctx, tx)
	})
}

// cerrarPorClima cierra todas las pistas abiertas (RN-15, CA-10). Las que ya cerró el operador
// siguen como están: sus vuelos ya perdieron el slot y no las reabre el clima (RN-16).
func (a *Pistas) cerrarPorClima(ctx context.Context, tx ports.Transaccion) error {
	pistas, err := tx.PistasConBloqueo(ctx)
	if err != nil {
		return err
	}
	var alternativos []domain.Alternativo
	for _, p := range pistas {
		if !p.Abierta {
			continue
		}
		if alternativos == nil {
			if alternativos, err = tx.Alternativos(ctx); err != nil {
				return err
			}
		}
		if err := a.cerrar(ctx, tx, p, domain.CerradaPorClima, alternativos); err != nil {
			return err
		}
	}
	return nil
}

// cerrar cierra una pista y libera los slots de ahí en adelante: el despegue queda demorado (RN-17,
// CA-12) y el arribo, desviado al primer alternativo disponible (RN-18, RN-25, CA-13).
func (a *Pistas) cerrar(ctx context.Context, tx ports.Transaccion, p domain.Pista, motivo domain.MotivoCierre, alternativos []domain.Alternativo) error {
	p.Abierta, p.CerradaPor = false, motivo
	if err := tx.GuardarPista(ctx, p); err != nil {
		return err
	}
	if err := tx.GuardarEvento(ctx, a.evento(ctx, domain.EventoPistaCerrada,
		domain.DatosPista{PistaID: p.ID, Motivo: motivo})); err != nil {
		return err
	}

	slots, err := tx.SlotsDePista(ctx, p.ID)
	if err != nil {
		return err
	}
	ahora := a.reloj()
	liberados := 0
	for _, s := range slots {
		if !domain.AfectadoPorCierre(s, ahora) {
			continue
		}
		var desvio *string
		if s.TipoOperacion == domain.Arribo {
			destino := domain.DestinoDelDesvio(alternativos)
			desvio = &destino
		}
		if err := tx.Liberar(ctx, s.ID); err != nil {
			return err
		}
		if err := tx.GuardarEvento(ctx, a.evento(ctx, domain.EventoSlotLiberado, domain.DatosSlotLiberado{
			VueloID:      s.VueloID,
			PistaID:      s.PistaID,
			InicioBloque: s.InicioBloque.UTC(),
			DesviadoA:    desvio,
		})); err != nil {
			return err
		}
		liberados++
	}
	slog.InfoContext(ctx, "pista cerrada", "runway_id", p.ID, "reason", motivo, "slots_liberados", liberados)
	return nil
}

// reabrirPorClima reabre solo las pistas que cerró el clima. Una cerrada por el operador sigue
// cerrada hasta que él la reabra (RN-16, CA-24).
func (a *Pistas) reabrirPorClima(ctx context.Context, tx ports.Transaccion) error {
	pistas, err := tx.PistasConBloqueo(ctx)
	if err != nil {
		return err
	}
	for _, p := range pistas {
		if p.Abierta || p.CerradaPor != domain.CerradaPorClima {
			continue
		}
		p.Abierta, p.CerradaPor = true, ""
		if err := tx.GuardarPista(ctx, p); err != nil {
			return err
		}
		if err := tx.GuardarEvento(ctx, a.evento(ctx, domain.EventoPistaReabierta,
			domain.DatosPista{PistaID: p.ID, Motivo: domain.CerradaPorClima})); err != nil {
			return err
		}
		slog.InfoContext(ctx, "pista reabierta", "runway_id", p.ID, "reason", domain.CerradaPorClima)
	}
	return nil
}
