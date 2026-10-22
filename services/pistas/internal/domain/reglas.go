package domain

import (
	"sort"
	"strings"
	"time"
)

// CantidadDeLibresCercanos es cuántos slots libres se informan al rechazar un pedido (18.2).
const CantidadDeLibresCercanos = 3

// ValidarPedido revisa que el pedido esté completo y que el bloque exista.
func ValidarPedido(p PedidoDeSlot) error {
	switch {
	case strings.TrimSpace(p.VueloID) == "":
		return ErrDatosInvalidos{Motivo: "falta vuelo_id"}
	case !p.TipoOperacion.Valido():
		return ErrDatosInvalidos{Motivo: "tipo_operacion tiene que ser arribo o despegue"}
	case p.PistaID <= 0:
		return ErrDatosInvalidos{Motivo: "falta pista_id"}
	case !InicioDeBloqueValido(p.InicioBloque):
		return ErrDatosInvalidos{Motivo: "el bloque tiene que empezar en :00, :15, :30 o :45"}
	}
	return nil
}

// InicioDeBloqueValido indica si el horario es el comienzo de un bloque de 15 minutos.
func InicioDeBloqueValido(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	t = t.In(ZonaCordoba)
	return t.Minute()%int(DuracionBloque/time.Minute) == 0 && t.Second() == 0 && t.Nanosecond() == 0
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
// cualquier pista abierta (18.2). No incluye el slot pedido. Busca hasta "limite" de distancia
// para cada lado.
//
// Orden: primero el más cercano en el tiempo; a igual distancia, el anterior antes que el
// posterior; en el mismo horario, la pista de menor id.
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
			libres = append(libres, SlotLibre{PistaID: p.ID, InicioBloque: t.In(ZonaCordoba)})
		}
	}

	agregar(origen)
	for paso := DuracionBloque; paso <= limite && len(libres) < n; paso += DuracionBloque {
		agregar(origen.Add(-paso))
		agregar(origen.Add(paso))
	}
	return libres
}

// BloquesDelDia devuelve el comienzo de cada bloque de un día, en hora de Córdoba.
func BloquesDelDia(fecha time.Time) []time.Time {
	inicio := time.Date(fecha.Year(), fecha.Month(), fecha.Day(), 0, 0, 0, 0, ZonaCordoba)
	fin := inicio.AddDate(0, 0, 1)
	var bloques []time.Time
	for t := inicio; t.Before(fin); t = t.Add(DuracionBloque) {
		bloques = append(bloques, t)
	}
	return bloques
}
