package domain

import (
	"sort"
	"strings"
	"time"
)

// CantidadDeLibresCercanos es cuántos slots libres se informan al rechazar un pedido (RN-09).
const CantidadDeLibresCercanos = 3

// ValidarPedido revisa que el pedido esté completo y que el bloque exista.
func ValidarPedido(p PedidoDeSlot) error {
	switch {
	case strings.TrimSpace(p.VueloID) == "":
		return ErrDatosInvalidos{Motivo: "falta flight_id"}
	case !p.TipoOperacion.Valido():
		return ErrDatosInvalidos{Motivo: "flight_type tiene que ser arrival o departure"}
	case strings.TrimSpace(p.PistaID) == "":
		return ErrDatosInvalidos{Motivo: "falta runway_id"}
	case !InicioDeBloqueValido(p.InicioBloque):
		return ErrDatosInvalidos{Motivo: "block_start tiene que empezar en :00, :15, :30 o :45"}
	}
	return nil
}

// InicioDeBloqueValido indica si el horario es el comienzo de un bloque de 15 minutos.
func InicioDeBloqueValido(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	return t.UTC().Truncate(DuracionBloque).Equal(t)
}

// Decision es qué hacer con un pedido según quién ocupa el slot.
type Decision int

const (
	// Ocupar: el slot está libre y el vuelo se lo queda (RN-08).
	Ocupar Decision = iota
	// Ceder: un arribo le quita el slot a un despegue (RN-11).
	Ceder
	// Rechazar: el slot está ocupado y el que pide no tiene prioridad (RN-09, RN-13, RN-14).
	Rechazar
)

// Decidir aplica la prioridad de los arribos. ocupante es nil si el slot está libre.
func Decidir(tipoDelPedido TipoOperacion, ocupante *Slot) Decision {
	if ocupante == nil {
		return Ocupar
	}
	if tipoDelPedido == Arribo && ocupante.TipoOperacion == Despegue {
		return Ceder
	}
	return Rechazar
}

// SlotsLibresCercanos busca los n slots libres más cercanos al pedido, antes o después, en
// cualquier pista abierta (RN-09). No incluye el slot pedido. Busca hasta "limite" de distancia
// para cada lado.
//
// Orden: primero el más cercano en el tiempo; a igual distancia, el anterior antes que el
// posterior; en el mismo horario, las pistas por nombre.
func SlotsLibresCercanos(pedido SlotLibre, pistas []Pista, ocupados []Slot, n int, limite time.Duration) []SlotLibre {
	abiertas := make([]Pista, 0, len(pistas))
	for _, p := range pistas {
		if p.Abierta {
			abiertas = append(abiertas, p)
		}
	}
	sort.Slice(abiertas, func(i, j int) bool { return abiertas[i].ID < abiertas[j].ID })

	tomado := make(map[SlotLibre]bool, len(ocupados))
	for _, s := range ocupados {
		tomado[SlotLibre{PistaID: s.PistaID, InicioBloque: s.InicioBloque.UTC()}] = true
	}

	origen := pedido.InicioBloque.UTC()
	libres := make([]SlotLibre, 0, n)
	agregar := func(t time.Time) {
		for _, p := range abiertas {
			if len(libres) == n {
				return
			}
			candidato := SlotLibre{PistaID: p.ID, InicioBloque: t}
			if tomado[candidato] || (p.ID == pedido.PistaID && t.Equal(origen)) {
				continue
			}
			libres = append(libres, candidato)
		}
	}

	agregar(origen)
	for paso := DuracionBloque; paso <= limite && len(libres) < n; paso += DuracionBloque {
		agregar(origen.Add(-paso))
		agregar(origen.Add(paso))
	}
	return libres
}
