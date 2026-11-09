//go:build integration

// Test de integración: prueba la operación crítica contra un MySQL real, que levanta
// testcontainers-go en Docker. Correr con:
//
//	go test -tags integration ./internal/adapters/mysql/
package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

func levantarBase(t *testing.T) *Repositorio {
	t.Helper()
	ctx := context.Background()
	contenedor, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase("pistas"),
		tcmysql.WithUsername("pistas"),
		tcmysql.WithPassword("pistas"),
		// El mismo script que usa el Docker Compose (5.6).
		tcmysql.WithScripts("../../../db/001_schema.sql"),
	)
	if err != nil {
		t.Fatalf("no se pudo levantar MySQL (¿está Docker corriendo?): %v", err)
	}
	t.Cleanup(func() { _ = contenedor.Terminate(ctx) })

	dsn, err := contenedor.ConnectionString(ctx, "parseTime=true", "loc=UTC")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := Conectar(dsn)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

var bloque = time.Date(2026, 10, 23, 17, 0, 0, 0, time.UTC)

// CA-05 y RN-10: muchos despegues piden el mismo slot a la vez; lo obtiene uno solo y los demás
// reciben un rechazo. Nunca quedan dos vuelos con el mismo slot.
func TestPedidosSimultaneosPorElMismoSlot(t *testing.T) {
	repo := levantarBase(t)
	pistas := aplicacion.NuevoPistas(repo)
	const n = 20

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = pistas.Asignar(context.Background(), domain.PedidoDeSlot{
				VueloID: fmt.Sprintf("D%02d", i), TipoOperacion: domain.Despegue, PistaID: "05-23", InicioBloque: bloque,
			})
		}(i)
	}
	wg.Wait()

	asignados, rechazados := 0, 0
	for _, err := range errs {
		var ocupado domain.ErrSlotOcupado
		switch {
		case err == nil:
			asignados++
		case errors.As(err, &ocupado):
			rechazados++
		default:
			t.Errorf("error inesperado: %v", err)
		}
	}
	if asignados != 1 || rechazados != n-1 {
		t.Fatalf("se esperaba 1 asignado y %d rechazados, hubo %d y %d", n-1, asignados, rechazados)
	}
	verificarUnSoloVuelo(t, repo)
}

// CA-08: un arribo y varios despegues piden el mismo slot a la vez; termina siendo del arribo.
func TestArriboYDespeguesSimultaneos(t *testing.T) {
	repo := levantarBase(t)
	pistas := aplicacion.NuevoPistas(repo)

	var wg sync.WaitGroup
	pedir := func(vuelo string, tipo domain.TipoOperacion) {
		defer wg.Done()
		_, _ = pistas.Asignar(context.Background(), domain.PedidoDeSlot{
			VueloID: vuelo, TipoOperacion: tipo, PistaID: "05-23", InicioBloque: bloque,
		})
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go pedir(fmt.Sprintf("D%02d", i), domain.Despegue)
	}
	wg.Add(1)
	go pedir("A01", domain.Arribo)
	wg.Wait()

	ocupados := verificarUnSoloVuelo(t, repo)
	if ocupados[0].VueloID != "A01" {
		t.Fatalf("el slot tenía que quedar para el arribo, quedó para %s", ocupados[0].VueloID)
	}
}

// CA-06: el mismo pedido enviado varias veces a la vez ocupa un solo slot.
func TestMismoPedidoRepetidoAlMismoTiempo(t *testing.T) {
	repo := levantarBase(t)
	pistas := aplicacion.NuevoPistas(repo)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := pistas.Asignar(context.Background(), domain.PedidoDeSlot{
				VueloID: "AR1550", TipoOperacion: domain.Arribo, PistaID: "05-23", InicioBloque: bloque,
			}); err != nil {
				t.Errorf("un pedido repetido no debería fallar: %v", err)
			}
		}()
	}
	wg.Wait()
	verificarUnSoloVuelo(t, repo)
}

func verificarUnSoloVuelo(t *testing.T, repo *Repositorio) []domain.Slot {
	t.Helper()
	ocupados, err := repo.SlotsOcupados(context.Background(), bloque, bloque.Add(domain.DuracionBloque))
	if err != nil {
		t.Fatal(err)
	}
	if len(ocupados) != 1 {
		t.Fatalf("el slot tiene que tener exactamente un vuelo, tiene %d", len(ocupados))
	}
	return ocupados
}

// RN-23 y 4.9: "vuelo cancelado" libera el slot, y el mismo mensaje dos veces a la vez se aplica una
// sola vez, porque la base no deja guardar dos veces su id.
func TestVueloCanceladoContraMySQL(t *testing.T) {
	repo := levantarBase(t)
	pistas := aplicacion.NuevoPistas(repo)
	if _, err := pistas.Asignar(context.Background(), domain.PedidoDeSlot{
		VueloID: "LA2000", TipoOperacion: domain.Despegue, PistaID: "05-23", InicioBloque: bloque,
	}); err != nil {
		t.Fatal(err)
	}

	mensaje := aplicacion.Mensaje{ID: "0c8f3d7e-6a51-4c1b-9d2e-5f4a3b2c1d0e", Type: aplicacion.EventoVueloCancelado,
		Data: []byte(`{"flight_id":"LA2000"}`)}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := pistas.ProcesarEvento(context.Background(), mensaje); err != nil && !errors.Is(err, ports.ErrConflicto) {
				t.Errorf("no se esperaba error: %v", err)
			}
		}()
	}
	wg.Wait()

	ocupados, err := repo.SlotsOcupados(context.Background(), bloque, bloque.Add(domain.DuracionBloque))
	if err != nil {
		t.Fatal(err)
	}
	if len(ocupados) != 0 {
		t.Fatalf("el slot tenía que quedar libre, tiene %d vuelos", len(ocupados))
	}
}

// manana es un bloque que todavía no empezó, para que el cierre lo alcance (pendiente 3).
var manana = time.Now().UTC().Add(24 * time.Hour).Truncate(domain.DuracionBloque)

func eventoDeClima(id string, operable bool) aplicacion.Mensaje {
	return aplicacion.Mensaje{ID: id, Type: aplicacion.EventoAptitudCambio,
		Data: []byte(fmt.Sprintf(`{"airport":"SACO","operable":%t,"reasons":[]}`, operable))}
}

func pendientesPorTipo(t *testing.T, repo *Repositorio) map[string][]ports.EventoGuardado {
	t.Helper()
	pendientes, err := repo.Pendientes(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	res := map[string][]ports.EventoGuardado{}
	for _, e := range pendientes {
		res[e.Tipo] = append(res[e.Tipo], e)
	}
	return res
}

// RN-15, RN-17, RN-18, RN-25 y RN-16 contra MySQL: el clima cierra las dos pistas, libera los
// slots con sus eventos en el outbox, y al mejorar reabre solo las que cerró él.
func TestCierrePorClimaContraMySQL(t *testing.T) {
	repo := levantarBase(t)
	pistas := aplicacion.NuevoPistas(repo)
	ctx := context.Background()
	for _, p := range []domain.PedidoDeSlot{
		{VueloID: "D01", TipoOperacion: domain.Despegue, PistaID: "05-23", InicioBloque: manana},
		{VueloID: "A01", TipoOperacion: domain.Arribo, PistaID: "18-36", InicioBloque: manana},
	} {
		if _, err := pistas.Asignar(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	if err := pistas.ProcesarEvento(ctx, eventoDeClima("6b0f1c2e-1111-4c1b-9d2e-5f4a3b2c1d0e", false)); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}

	todas, _ := repo.Pistas(ctx)
	for _, p := range todas {
		if p.Abierta || p.CerradaPor != domain.CerradaPorClima {
			t.Errorf("la pista %s tenía que quedar cerrada por el clima: %+v", p.ID, p)
		}
	}
	if ocupados, _ := repo.SlotsOcupados(ctx, manana, manana.Add(domain.DuracionBloque)); len(ocupados) != 0 {
		t.Errorf("los slots tenían que quedar libres: %+v", ocupados)
	}
	eventos := pendientesPorTipo(t, repo)
	if len(eventos[domain.EventoPistaCerrada]) != 2 || len(eventos[domain.EventoSlotLiberado]) != 2 {
		t.Fatalf("faltan eventos en el outbox: %d pista.cerrada y %d slot.liberado",
			len(eventos[domain.EventoPistaCerrada]), len(eventos[domain.EventoSlotLiberado]))
	}
	for _, e := range eventos[domain.EventoSlotLiberado] {
		var d struct {
			FlightID   string  `json:"flight_id"`
			DivertedTo *string `json:"diverted_to"`
		}
		_ = json.Unmarshal(e.DatosJSON, &d)
		if d.FlightID == "A01" && (d.DivertedTo == nil || *d.DivertedTo != "SAOU") {
			t.Errorf("el arribo tenía que ir a SAOU: %s", e.DatosJSON)
		}
		if d.FlightID == "D01" && d.DivertedTo != nil {
			t.Errorf("el despegue no se desvía: %s", e.DatosJSON)
		}
	}

	// El operador cierra 18-36 a mano: el clima no la reabre (RN-16).
	if err := repo.EnTransaccion(ctx, func(ctx context.Context, tx ports.Transaccion) error {
		return tx.GuardarPista(ctx, domain.Pista{ID: "18-36", Abierta: false, CerradaPor: domain.CerradaPorOperador})
	}); err != nil {
		t.Fatal(err)
	}
	if err := pistas.ProcesarEvento(ctx, eventoDeClima("6b0f1c2e-2222-4c1b-9d2e-5f4a3b2c1d0e", true)); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	todas, _ = repo.Pistas(ctx)
	if !todas[0].Abierta || todas[1].Abierta || todas[1].CerradaPor != domain.CerradaPorOperador {
		t.Errorf("se esperaba 05-23 abierta y 18-36 cerrada por el operador: %+v", todas)
	}
}

// ADR-004: un cierre por clima que llega mientras se piden slots. Ningún slot queda ocupado en una
// pista cerrada, y cada vuelo que lo había conseguido recibe su "slot liberado".
func TestCierreConPedidosSimultaneos(t *testing.T) {
	repo := levantarBase(t)
	pistas := aplicacion.NuevoPistas(repo)
	ctx := context.Background()
	const n = 20

	var wg sync.WaitGroup
	asignados := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := pistas.Asignar(ctx, domain.PedidoDeSlot{
				VueloID: fmt.Sprintf("D%02d", i), TipoOperacion: domain.Despegue,
				PistaID: []string{"05-23", "18-36"}[i%2], InicioBloque: manana.Add(time.Duration(i) * domain.DuracionBloque),
			})
			switch {
			case err == nil:
				asignados[i] = true
			case !errors.Is(err, domain.ErrPistaCerrada) && !errors.Is(err, ports.ErrConflicto):
				t.Errorf("error inesperado: %v", err)
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			err := pistas.ProcesarEvento(ctx, eventoDeClima("6b0f1c2e-3333-4c1b-9d2e-5f4a3b2c1d0e", false))
			if !errors.Is(err, ports.ErrConflicto) {
				if err != nil {
					t.Errorf("no se esperaba error al cerrar: %v", err)
				}
				return
			}
		}
	}()
	wg.Wait()

	if ocupados, _ := repo.SlotsOcupados(ctx, manana, manana.Add(n*domain.DuracionBloque)); len(ocupados) != 0 {
		t.Fatalf("quedaron slots ocupados en pistas cerradas: %+v", ocupados)
	}
	conSlot := 0
	for _, ok := range asignados {
		if ok {
			conSlot++
		}
	}
	if liberados := len(pendientesPorTipo(t, repo)[domain.EventoSlotLiberado]); liberados != conSlot {
		t.Fatalf("%d vuelos consiguieron slot pero se liberaron %d", conSlot, liberados)
	}
}
