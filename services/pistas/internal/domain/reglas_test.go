package domain

import (
	"errors"
	"testing"
	"time"
)

func hora(h, m int) time.Time {
	return time.Date(2026, 10, 23, h, m, 0, 0, ZonaCordoba)
}

func TestInicioDeBloqueValido(t *testing.T) {
	casos := []struct {
		nombre string
		t      time.Time
		valido bool
	}{
		{"en punto", hora(14, 0), true},
		{"y cuarto", hora(14, 15), true},
		{"menos cuarto", hora(14, 45), true},
		{"minuto suelto", hora(14, 10), false},
		{"con segundos", hora(14, 15).Add(30 * time.Second), false},
		{"en UTC, mismo instante", hora(14, 15).UTC(), true},
		{"vacío", time.Time{}, false},
	}
	for _, c := range casos {
		if got := InicioDeBloqueValido(c.t); got != c.valido {
			t.Errorf("%s: InicioDeBloqueValido = %v, se esperaba %v", c.nombre, got, c.valido)
		}
	}
}

func TestValidarPedido(t *testing.T) {
	bueno := PedidoDeSlot{VueloID: "v1", TipoOperacion: Arribo, PistaID: 1, InicioBloque: hora(10, 0)}
	if err := ValidarPedido(bueno); err != nil {
		t.Fatalf("un pedido completo no debería fallar: %v", err)
	}

	malos := map[string]func(p *PedidoDeSlot){
		"sin vuelo":      func(p *PedidoDeSlot) { p.VueloID = " " },
		"tipo inválido":  func(p *PedidoDeSlot) { p.TipoOperacion = "aterrizaje" },
		"sin pista":      func(p *PedidoDeSlot) { p.PistaID = 0 },
		"bloque partido": func(p *PedidoDeSlot) { p.InicioBloque = hora(10, 7) },
	}
	for nombre, romper := range malos {
		p := bueno
		romper(&p)
		var e ErrDatosInvalidos
		if err := ValidarPedido(p); !errors.As(err, &e) {
			t.Errorf("%s: se esperaba ErrDatosInvalidos, vino %v", nombre, err)
		}
	}
}

// La prioridad de los arribos: RN-11, RN-13 y RN-14.
func TestDecidir(t *testing.T) {
	arribo := &Slot{TipoOperacion: Arribo}
	despegue := &Slot{TipoOperacion: Despegue}
	casos := []struct {
		nombre   string
		pedido   TipoOperacion
		ocupante *Slot
		esperado Decision
	}{
		{"arribo pide un slot libre", Arribo, nil, Ocupar},
		{"despegue pide un slot libre", Despegue, nil, Ocupar},
		{"arribo pide el slot de un despegue: se lo queda (RN-11)", Arribo, despegue, Ceder},
		{"arribo pide el slot de otro arribo: gana el primero (RN-13)", Arribo, arribo, Rechazar},
		{"despegue pide el slot de otro despegue: gana el primero (RN-13)", Despegue, despegue, Rechazar},
		{"despegue pide el slot de un arribo: nunca quita (RN-14)", Despegue, arribo, Rechazar},
	}
	for _, c := range casos {
		if got := Decidir(c.pedido, c.ocupante); got != c.esperado {
			t.Errorf("%s: Decidir = %v, se esperaba %v", c.nombre, got, c.esperado)
		}
	}
}

func TestSlotsLibresCercanos(t *testing.T) {
	pistas := []Pista{{ID: 2, Abierta: true}, {ID: 1, Abierta: true}}
	pedido := SlotLibre{PistaID: 1, InicioBloque: hora(14, 0)}

	t.Run("primero el mismo horario en otra pista, después antes y después", func(t *testing.T) {
		ocupados := []Slot{{PistaID: 1, InicioBloque: hora(14, 0)}}
		got := SlotsLibresCercanos(pedido, pistas, ocupados, 3, 24*time.Hour)
		esperado := []SlotLibre{
			{PistaID: 2, InicioBloque: hora(14, 0)},
			{PistaID: 1, InicioBloque: hora(13, 45)},
			{PistaID: 2, InicioBloque: hora(13, 45)},
		}
		compararLibres(t, got, esperado)
	})

	t.Run("saltea los ocupados y las pistas cerradas", func(t *testing.T) {
		conCerrada := []Pista{{ID: 1, Abierta: true}, {ID: 2, Abierta: false}}
		ocupados := []Slot{
			{PistaID: 1, InicioBloque: hora(14, 0)},
			{PistaID: 1, InicioBloque: hora(13, 45)},
		}
		got := SlotsLibresCercanos(pedido, conCerrada, ocupados, 3, 24*time.Hour)
		esperado := []SlotLibre{
			{PistaID: 1, InicioBloque: hora(14, 15)},
			{PistaID: 1, InicioBloque: hora(13, 30)},
			{PistaID: 1, InicioBloque: hora(14, 30)},
		}
		compararLibres(t, got, esperado)
	})

	t.Run("no pasa del límite", func(t *testing.T) {
		soloUna := []Pista{{ID: 1, Abierta: true}}
		got := SlotsLibresCercanos(pedido, soloUna, nil, 3, 15*time.Minute)
		if len(got) != 2 {
			t.Fatalf("con un límite de un bloque para cada lado hay 2 libres, vinieron %d: %v", len(got), got)
		}
	})

	t.Run("no ofrece el slot pedido aunque esté libre", func(t *testing.T) {
		got := SlotsLibresCercanos(pedido, pistas, nil, 3, 24*time.Hour)
		for _, l := range got {
			if l.PistaID == pedido.PistaID && l.InicioBloque.Equal(pedido.InicioBloque) {
				t.Fatalf("ofreció el mismo slot que se pidió: %v", got)
			}
		}
	})
}

func TestBloquesDelDia(t *testing.T) {
	bloques := BloquesDelDia(hora(9, 0))
	if len(bloques) != 96 {
		t.Fatalf("un día tiene 96 bloques de 15 minutos, vinieron %d", len(bloques))
	}
	if !bloques[0].Equal(hora(0, 0)) || !bloques[95].Equal(hora(23, 45)) {
		t.Fatalf("el día va de 00:00 a 23:45, vino de %v a %v", bloques[0], bloques[95])
	}
}

func compararLibres(t *testing.T, got, esperado []SlotLibre) {
	t.Helper()
	if len(got) != len(esperado) {
		t.Fatalf("vinieron %d libres, se esperaban %d: %v", len(got), len(esperado), got)
	}
	for i := range esperado {
		if got[i].PistaID != esperado[i].PistaID || !got[i].InicioBloque.Equal(esperado[i].InicioBloque) {
			t.Errorf("libre %d: vino pista %d a las %s, se esperaba pista %d a las %s", i,
				got[i].PistaID, got[i].InicioBloque.Format("15:04"),
				esperado[i].PistaID, esperado[i].InicioBloque.Format("15:04"))
		}
	}
}
