package aplicacion

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

func cancelado(id, vuelo string) Mensaje {
	datos, _ := json.Marshal(map[string]string{"flight_id": vuelo})
	return Mensaje{ID: id, Type: EventoVueloCancelado, Data: datos}
}

// RN-23 y CA-17: cuando un vuelo se cancela, su slot queda libre y otro vuelo lo puede pedir.
func TestVueloCanceladoLiberaSuSlot(t *testing.T) {
	repo := nuevoRepo()
	p := NuevoPistas(repo)
	_, _ = p.Asignar(context.Background(), pedido("LA2000", domain.Despegue, "05-23", catorce))

	if err := p.ProcesarEvento(context.Background(), cancelado("m1", "LA2000")); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(repo.slots) != 0 {
		t.Fatalf("el slot tenía que quedar libre: %+v", repo.slots)
	}
	if _, err := p.Asignar(context.Background(), pedido("AR1000", domain.Despegue, "05-23", catorce)); err != nil {
		t.Fatalf("otro vuelo tiene que poder pedir el slot liberado: %v", err)
	}
}

// 4.9: el mismo mensaje dos veces se aplica una sola vez. Si en el medio otro vuelo tomó el slot,
// la repetición no lo toca.
func TestVueloCanceladoRepetidoSeAplicaUnaVez(t *testing.T) {
	repo := nuevoRepo()
	p := NuevoPistas(repo)
	_, _ = p.Asignar(context.Background(), pedido("LA2000", domain.Despegue, "05-23", catorce))
	_ = p.ProcesarEvento(context.Background(), cancelado("m1", "LA2000"))
	_, _ = p.Asignar(context.Background(), pedido("LA2000", domain.Despegue, "18-36", catorce))

	if err := p.ProcesarEvento(context.Background(), cancelado("m1", "LA2000")); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(repo.slots) != 1 {
		t.Fatalf("la repetición no tenía que liberar nada: %+v", repo.slots)
	}
}

func TestVueloCanceladoSinSlotNoHaceNada(t *testing.T) {
	repo := nuevoRepo()
	if err := NuevoPistas(repo).ProcesarEvento(context.Background(), cancelado("m1", "sin-slot")); err != nil {
		t.Fatalf("un vuelo cancelado sin slot no es un error: %v", err)
	}
	if !repo.procesados["m1"] {
		t.Fatalf("el mensaje tiene que quedar registrado como procesado")
	}
}

// 4.22: un evento sin flight_id no se arregla reintentando.
func TestVueloCanceladoSinDatosEsInaplicable(t *testing.T) {
	err := NuevoPistas(nuevoRepo()).ProcesarEvento(context.Background(),
		Mensaje{ID: "m1", Type: EventoVueloCancelado, Data: json.RawMessage(`{}`)})
	if !errors.Is(err, ErrInaplicable) {
		t.Fatalf("se esperaba ErrInaplicable, vino %v", err)
	}
}

// Los eventos de clima y los desconocidos se confirman sin cambiar nada (8.1, 4.22).
func TestEventosSinEfecto(t *testing.T) {
	repo := nuevoRepo()
	p := NuevoPistas(repo)
	_, _ = p.Asignar(context.Background(), pedido("LA2000", domain.Despegue, "05-23", catorce))
	for _, tipo := range []string{"clima.aptitud.cambio", "clima.sin_datos", "otro.evento"} {
		if err := p.ProcesarEvento(context.Background(), Mensaje{ID: tipo, Type: tipo}); err != nil {
			t.Errorf("%s: no se esperaba error: %v", tipo, err)
		}
	}
	if len(repo.slots) != 1 {
		t.Fatalf("no tenían que cambiar los slots: %+v", repo.slots)
	}
}
