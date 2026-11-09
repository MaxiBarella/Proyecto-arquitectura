package aplicacion

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

func aptitud(id string, operable bool) Mensaje {
	datos, _ := json.Marshal(map[string]any{"airport": "SACO", "operable": operable, "reasons": []string{}})
	return Mensaje{ID: id, Type: EventoAptitudCambio, Data: datos}
}

func sinDatos(id string) Mensaje {
	datos, _ := json.Marshal(map[string]any{"airport": "SACO"})
	return Mensaje{ID: id, Type: EventoClimaSinDatos, Data: datos}
}

// pistasConVuelos arma un repositorio con un despegue y un arribo en 05-23 y un despegue en 18-36,
// todos después de "ahora", y un despegue en 05-23 cuyo bloque ya empezó.
func pistasConVuelos(t *testing.T) (*repoEnMemoria, *Pistas) {
	t.Helper()
	repo := nuevoRepo()
	p := NuevoPistas(repo)
	ctx := context.Background()
	for _, pd := range []domain.PedidoDeSlot{
		pedido("EN-CURSO", domain.Despegue, "05-23", catorce.Add(-15*time.Minute)),
		pedido("DESPEGUE", domain.Despegue, "05-23", catorce),
		pedido("ARRIBO", domain.Arribo, "05-23", catorce.Add(15*time.Minute)),
		pedido("OTRA-PISTA", domain.Despegue, "18-36", catorce),
	} {
		if _, err := p.Asignar(ctx, pd); err != nil {
			t.Fatal(err)
		}
	}
	repo.eventos = nil
	// Son 10 minutos antes de catorce: el bloque anterior ya empezó.
	p.reloj = func() time.Time { return catorce.Add(-10 * time.Minute) }
	return repo, p
}

func liberados(eventos []domain.Evento) map[string]*string {
	res := map[string]*string{}
	for _, e := range eventos {
		if d, ok := e.Datos.(domain.DatosSlotLiberado); ok {
			res[d.VueloID] = d.DesviadoA
		}
	}
	return res
}

// RN-15, CA-10, RN-17, RN-18, CA-12, CA-13 y CA-21: cuando el clima no deja operar, se cierran
// todas las pistas, el despegue queda demorado y el arribo va al alternativo más cercano.
func TestClimaCierraLasPistas(t *testing.T) {
	repo, p := pistasConVuelos(t)

	if err := p.ProcesarEvento(context.Background(), aptitud("m1", false)); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}

	for _, pista := range repo.pistas {
		if pista.Abierta || pista.CerradaPor != domain.CerradaPorClima {
			t.Errorf("la pista %s tenía que quedar cerrada por el clima: %+v", pista.ID, pista)
		}
	}
	lib := liberados(repo.eventos)
	if len(lib) != 3 {
		t.Fatalf("se esperaban 3 slots liberados, hubo %d: %v", len(lib), tiposDeEvento(repo.eventos))
	}
	if d, ok := lib["DESPEGUE"]; !ok || d != nil {
		t.Errorf("el despegue tenía que liberarse sin desvío: %v", d)
	}
	if d := lib["ARRIBO"]; d == nil || *d != "SAOU" {
		t.Errorf("el arribo tenía que desviarse a SAOU: %v", d)
	}
	if len(repo.slots) != 1 || repo.slots[0].VueloID != "EN-CURSO" {
		t.Errorf("solo tenía que quedar el vuelo cuyo bloque ya empezó (pendiente 3): %+v", repo.slots)
	}
	if !repo.procesados["m1"] || repo.clima.Operable == nil || *repo.clima.Operable {
		t.Errorf("el estado del clima tenía que quedar no operable: %+v", repo.clima)
	}
	cerradas := 0
	for _, e := range repo.eventos {
		if d, ok := e.Datos.(domain.DatosPista); ok && e.Tipo == domain.EventoPistaCerrada && d.Motivo == domain.CerradaPorClima {
			cerradas++
		}
	}
	if cerradas != 2 {
		t.Errorf("se esperaban dos pista.cerrada por el clima, hubo %d", cerradas)
	}
}

// RN-26 y CA-22: si el más cercano no está disponible, el arribo va al siguiente.
func TestClimaDesviaAlSiguienteDisponible(t *testing.T) {
	repo, p := pistasConVuelos(t)
	repo.alternativos[0].Disponible = false

	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))

	if d := liberados(repo.eventos)["ARRIBO"]; d == nil || *d != "SAAR" {
		t.Errorf("el arribo tenía que desviarse a SAAR: %v", d)
	}
}

// RN-16 y CA-24: una pista cerrada a mano no la reabre el clima, y el cierre por clima no le
// cambia el motivo.
func TestClimaNoReabreLaCerradaPorElOperador(t *testing.T) {
	repo, p := pistasConVuelos(t)
	repo.pistas[1] = domain.Pista{ID: "18-36", Abierta: false, CerradaPor: domain.CerradaPorOperador}

	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))
	if repo.pistas[1].CerradaPor != domain.CerradaPorOperador {
		t.Fatalf("la cerrada por el operador tenía que seguir así: %+v", repo.pistas[1])
	}
	_ = p.ProcesarEvento(context.Background(), aptitud("m2", true))

	if !repo.pistas[0].Abierta {
		t.Errorf("la cerrada por el clima tenía que reabrirse: %+v", repo.pistas[0])
	}
	if repo.pistas[1].Abierta {
		t.Errorf("la cerrada por el operador no la reabre el clima: %+v", repo.pistas[1])
	}
}

// Al reabrir se publica "pista reabierta" por cada pista que había cerrado el clima.
func TestClimaReabreLasPistas(t *testing.T) {
	repo, p := pistasConVuelos(t)
	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))
	repo.eventos = nil

	if err := p.ProcesarEvento(context.Background(), aptitud("m2", true)); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}

	for _, pista := range repo.pistas {
		if !pista.Abierta || pista.CerradaPor != "" {
			t.Errorf("la pista %s tenía que quedar abierta: %+v", pista.ID, pista)
		}
	}
	tipos := tiposDeEvento(repo.eventos)
	if len(tipos) != 2 || tipos[0] != domain.EventoPistaReabierta || tipos[1] != domain.EventoPistaReabierta {
		t.Errorf("se esperaban dos pista.reabierta, hubo %v", tipos)
	}
}

// RN-24 y CA-20: sin datos de Clima las pistas no cambian, y cuando vuelven los datos se aplica
// lo que digan.
func TestClimaSinDatosNoCambiaLasPistas(t *testing.T) {
	repo, p := pistasConVuelos(t)

	if err := p.ProcesarEvento(context.Background(), sinDatos("m1")); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if repo.clima.ConDatos {
		t.Errorf("tenía que quedar registrado que Clima no tiene datos")
	}
	if len(repo.eventos) != 0 || !repo.pistas[0].Abierta || len(repo.slots) != 4 {
		t.Fatalf("sin datos no tenía que cambiar nada: %v", tiposDeEvento(repo.eventos))
	}

	_ = p.ProcesarEvento(context.Background(), aptitud("m2", false))
	if !repo.clima.ConDatos || repo.pistas[0].Abierta {
		t.Errorf("al volver los datos se tenía que aplicar el cierre: %+v %+v", repo.clima, repo.pistas)
	}
}

// 4.9: el mismo evento de clima dos veces se aplica una sola vez.
func TestClimaRepetidoSeAplicaUnaVez(t *testing.T) {
	repo, p := pistasConVuelos(t)
	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))
	antes := len(repo.eventos)

	if err := p.ProcesarEvento(context.Background(), aptitud("m1", false)); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(repo.eventos) != antes {
		t.Errorf("la repetición no tenía que generar eventos: %v", tiposDeEvento(repo.eventos))
	}
}

// RN-27 y CA-23: con la pista cerrada por el clima no se puede pedir un slot en ella.
func TestNoSePideSlotEnPistaCerradaPorClima(t *testing.T) {
	_, p := pistasConVuelos(t)
	_ = p.ProcesarEvento(context.Background(), aptitud("m1", false))

	_, err := p.Asignar(context.Background(), pedido("NUEVO", domain.Despegue, "05-23", catorce.Add(time.Hour)))
	if !errors.Is(err, domain.ErrPistaCerrada) {
		t.Fatalf("se esperaba ErrPistaCerrada, vino %v", err)
	}
}

// 4.22: un evento de aptitud sin "operable" no se arregla reintentando, y uno de otro aeropuerto
// se confirma sin cambiar nada (6.9).
func TestClimaConDatosRaros(t *testing.T) {
	repo, p := pistasConVuelos(t)

	err := p.ProcesarEvento(context.Background(),
		Mensaje{ID: "m1", Type: EventoAptitudCambio, Data: json.RawMessage(`{"airport":"SACO"}`)})
	if !errors.Is(err, ErrInaplicable) {
		t.Errorf("sin operable se esperaba ErrInaplicable, vino %v", err)
	}

	otro, _ := json.Marshal(map[string]any{"airport": "SAEZ", "operable": false})
	if err := p.ProcesarEvento(context.Background(), Mensaje{ID: "m2", Type: EventoAptitudCambio, Data: otro}); err != nil {
		t.Errorf("otro aeropuerto no es un error: %v", err)
	}
	if !repo.pistas[0].Abierta {
		t.Errorf("el clima de otro aeropuerto no tenía que cerrar nada")
	}
}
