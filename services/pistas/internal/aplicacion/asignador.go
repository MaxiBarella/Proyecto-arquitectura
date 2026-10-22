package aplicacion

import (
	"context"
	"errors"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// intentosPorConflicto es cuántas veces se corre la asignación si choca con otra simultánea.
// La segunda vez ya ve lo que guardó la otra y decide con eso (RN-10, CA-08).
const intentosPorConflicto = 3

// ventanasDeBusqueda son los rangos, hacia cada lado, en que se buscan los libres cercanos. Se
// agranda solo si no alcanzan; 18.2 dice "sin límite de distancia" y un año lo es en la práctica.
var ventanasDeBusqueda = []time.Duration{24 * time.Hour, 7 * 24 * time.Hour, 365 * 24 * time.Hour}

// Pistas reúne los casos de uso del servicio.
type Pistas struct {
	repo    ports.Repositorio
	reloj   func() time.Time
	nuevoID func() string
}

// NuevoPistas arma los casos de uso sobre un repositorio.
func NuevoPistas(repo ports.Repositorio) *Pistas {
	return &Pistas{repo: repo, reloj: time.Now, nuevoID: NuevoUUID}
}

// Asignar atiende un pedido de slot de Vuelos (4.1).
func (a *Pistas) Asignar(ctx context.Context, p domain.PedidoDeSlot) (domain.Resultado, error) {
	if err := domain.ValidarPedido(p); err != nil {
		return domain.Resultado{}, err
	}
	p.InicioBloque = p.InicioBloque.In(domain.ZonaCordoba)

	var res domain.Resultado
	var err error
	for intento := 1; intento <= intentosPorConflicto; intento++ {
		err = a.repo.EnTransaccion(ctx, func(ctx context.Context, tx ports.Transaccion) error {
			var errTx error
			res, errTx = a.asignarEn(ctx, tx, p)
			return errTx
		})
		if !errors.Is(err, ports.ErrConflicto) {
			break
		}
	}
	return res, err
}

func (a *Pistas) asignarEn(ctx context.Context, tx ports.Transaccion, p domain.PedidoDeSlot) (domain.Resultado, error) {
	pista, err := tx.Pista(ctx, p.PistaID)
	if err != nil {
		return domain.Resultado{}, err
	}
	if !pista.Abierta {
		return domain.Resultado{}, domain.ErrPistaCerrada
	}

	actual, err := tx.SlotDelVuelo(ctx, p.VueloID)
	if err != nil {
		return domain.Resultado{}, err
	}
	if actual != nil {
		if actual.MismoLugar(p.PistaID, p.InicioBloque) {
			return domain.Resultado{Slot: *actual, YaAsignado: true}, nil
		}
		return domain.Resultado{}, domain.ErrVueloConSlot
	}

	ocupante, err := tx.OcupanteConBloqueo(ctx, p.PistaID, p.InicioBloque)
	if err != nil {
		return domain.Resultado{}, err
	}

	switch domain.Decidir(p.TipoOperacion, ocupante) {
	case domain.Rechazar:
		libres, err := a.libresCercanos(ctx, tx, p)
		if err != nil {
			return domain.Resultado{}, err
		}
		return domain.Resultado{}, domain.ErrSlotOcupado{Libres: libres}

	case domain.Ceder:
		if err := tx.Liberar(ctx, ocupante.ID); err != nil {
			return domain.Resultado{}, err
		}
		cedido := a.evento(ctx, domain.EventoSlotCedido, domain.DatosSlotCedido{
			VueloID:        ocupante.VueloID,
			SlotID:         ocupante.ID,
			PistaID:        ocupante.PistaID,
			InicioBloque:   ocupante.InicioBloque,
			CedidoAVueloID: p.VueloID,
		})
		if err := tx.GuardarEvento(ctx, cedido); err != nil {
			return domain.Resultado{}, err
		}
		nuevo, err := a.ocupar(ctx, tx, p)
		if err != nil {
			return domain.Resultado{}, err
		}
		return domain.Resultado{Slot: nuevo, VueloDesplazadoID: ocupante.VueloID}, nil

	default:
		nuevo, err := a.ocupar(ctx, tx, p)
		if err != nil {
			return domain.Resultado{}, err
		}
		return domain.Resultado{Slot: nuevo}, nil
	}
}

// ocupar guarda el slot y su evento "slot.asignado".
func (a *Pistas) ocupar(ctx context.Context, tx ports.Transaccion, p domain.PedidoDeSlot) (domain.Slot, error) {
	s := domain.Slot{
		PistaID:       p.PistaID,
		InicioBloque:  p.InicioBloque,
		VueloID:       p.VueloID,
		TipoOperacion: p.TipoOperacion,
	}
	if err := tx.Ocupar(ctx, &s); err != nil {
		return domain.Slot{}, err
	}
	e := a.evento(ctx, domain.EventoSlotAsignado, domain.DatosSlotAsignado{
		VueloID:       s.VueloID,
		SlotID:        s.ID,
		PistaID:       s.PistaID,
		InicioBloque:  s.InicioBloque,
		TipoOperacion: s.TipoOperacion,
	})
	if err := tx.GuardarEvento(ctx, e); err != nil {
		return domain.Slot{}, err
	}
	return s, nil
}

func (a *Pistas) libresCercanos(ctx context.Context, tx ports.Transaccion, p domain.PedidoDeSlot) ([]domain.SlotLibre, error) {
	pistas, err := tx.Pistas(ctx)
	if err != nil {
		return nil, err
	}
	pedido := domain.SlotLibre{PistaID: p.PistaID, InicioBloque: p.InicioBloque}
	var libres []domain.SlotLibre
	for _, ventana := range ventanasDeBusqueda {
		ocupados, err := tx.SlotsOcupados(ctx, p.InicioBloque.Add(-ventana), p.InicioBloque.Add(ventana+domain.DuracionBloque))
		if err != nil {
			return nil, err
		}
		libres = domain.SlotsLibresCercanos(pedido, pistas, ocupados, domain.CantidadDeLibresCercanos, ventana)
		if len(libres) == domain.CantidadDeLibresCercanos {
			break
		}
	}
	return libres, nil
}

func (a *Pistas) evento(ctx context.Context, tipo string, datos any) domain.Evento {
	return domain.Evento{
		ID:            a.nuevoID(),
		Tipo:          tipo,
		Fecha:         a.reloj().In(domain.ZonaCordoba),
		CorrelacionID: CorrelacionDe(ctx),
		Datos:         datos,
	}
}

// BloqueDelDia es un bloque de una pista en la consulta de slots.
type BloqueDelDia struct {
	domain.SlotLibre
	Libre bool
}

// ListarSlots devuelve los bloques de un día de cada pista, o de una sola si pistaID no es 0.
// Un bloque de una pista cerrada no está libre: no se puede pedir (18.4).
func (a *Pistas) ListarSlots(ctx context.Context, fecha time.Time, pistaID int64) ([]BloqueDelDia, error) {
	pistas, err := a.repo.Pistas(ctx)
	if err != nil {
		return nil, err
	}
	bloques := domain.BloquesDelDia(fecha)
	ocupados, err := a.repo.SlotsOcupados(ctx, bloques[0], bloques[len(bloques)-1].Add(domain.DuracionBloque))
	if err != nil {
		return nil, err
	}
	tomado := make(map[domain.SlotLibre]bool, len(ocupados))
	for _, s := range ocupados {
		tomado[domain.SlotLibre{PistaID: s.PistaID, InicioBloque: s.InicioBloque.UTC()}] = true
	}

	var res []BloqueDelDia
	for _, p := range pistas {
		if pistaID != 0 && p.ID != pistaID {
			continue
		}
		for _, b := range bloques {
			libre := p.Abierta && !tomado[domain.SlotLibre{PistaID: p.ID, InicioBloque: b.UTC()}]
			res = append(res, BloqueDelDia{SlotLibre: domain.SlotLibre{PistaID: p.ID, InicioBloque: b}, Libre: libre})
		}
	}
	return res, nil
}

// ListarPistas devuelve todas las pistas.
func (a *Pistas) ListarPistas(ctx context.Context) ([]domain.Pista, error) {
	return a.repo.Pistas(ctx)
}
