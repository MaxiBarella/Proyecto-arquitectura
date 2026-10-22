//go:build integration

// Test de integración: prueba la operación crítica contra un MySQL real, que levanta
// testcontainers-go en Docker. Correr con:
//
//	go test -tags integration ./internal/adapters/mysql/
package mysql

import (
	"context"
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
