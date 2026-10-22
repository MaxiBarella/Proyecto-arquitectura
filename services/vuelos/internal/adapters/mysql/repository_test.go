package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	"gorm.io/gorm"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// Un solo MySQL real para todos los tests, con las mismas tablas que el Docker
// Compose (db/001_schema.sql). Si Docker no está disponible, los tests se saltean.
var (
	testDB    *gorm.DB
	skipCause string
)

func TestMain(m *testing.M) {
	code, err := runWithMySQL(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runWithMySQL(m *testing.M) (int, error) {
	ctx := context.Background()
	if err := dockerAvailable(); err != nil {
		skipCause = "Docker no está disponible: " + err.Error()
		return m.Run(), nil
	}
	container, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase("vuelos"),
		tcmysql.WithUsername("vuelos"),
		tcmysql.WithPassword("vuelos"),
		tcmysql.WithScripts("../../../db/001_schema.sql"),
	)
	if err != nil {
		return 0, fmt.Errorf("levantar MySQL: %w", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(container) }()

	dsn, err := container.ConnectionString(ctx, "parseTime=true", "loc=UTC")
	if err != nil {
		return 0, err
	}
	if testDB, err = Open(ctx, dsn); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

func dockerAvailable() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return err
	}
	defer provider.Close()
	return provider.Health(context.Background())
}

// newRepo vacía las tablas y devuelve un repositorio con un reloj fijo.
func newRepo(t *testing.T) *Repository {
	t.Helper()
	if testDB == nil {
		t.Skip(skipCause)
	}
	for _, table := range []string{"flights", "outbox", "processed_messages"} {
		if err := testDB.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("vaciar %s: %v", table, err)
		}
	}
	r := NewRepository(testDB)
	r.now = func() time.Time { return now }
	return r
}

var (
	ctx   = context.Background()
	now   = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	block = time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)
)

func newFlight(t *testing.T, airline, number string, scheduledAt time.Time) *domain.Flight {
	t.Helper()
	f, err := domain.NewFlight(domain.NewID(), airline, domain.NewFlightData{
		Number:      number,
		Type:        domain.Departure,
		Scope:       domain.Domestic,
		Origin:      "Córdoba",
		Destination: "Buenos Aires",
		ScheduledAt: scheduledAt,
	}, now)
	if err != nil {
		t.Fatalf("NewFlight: %v", err)
	}
	return f
}

func event(eventType string) domain.Event {
	return domain.Event{
		ID:            domain.NewID(),
		Type:          eventType,
		OccurredAt:    now,
		CorrelationID: "trace-1",
		Data:          map[string]string{"flight_id": "x"},
	}
}

func outbox(t *testing.T) []outboxRow {
	t.Helper()
	var rows []outboxRow
	if err := testDB.Order("occurred_at, id").Find(&rows).Error; err != nil {
		t.Fatalf("leer el outbox: %v", err)
	}
	return rows
}

func TestCreateAndGet(t *testing.T) {
	r := newRepo(t)
	f := newFlight(t, "AR", "AR1550", block)
	if err := r.Create(ctx, f); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := r.Get(ctx, f.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Number != "AR1550" || got.Airline != "AR" || got.Status != domain.PendingSlot ||
		got.Slot != nil || got.Version != 1 || !got.ScheduledAt.Equal(block) || !got.CreatedAt.Equal(now) {
		t.Errorf("el vuelo leído no coincide con el guardado: %+v", got)
	}
}

func TestGetNotFound(t *testing.T) {
	r := newRepo(t)
	if _, err := r.Get(ctx, domain.NewID()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestCreateSavesEventsInTheSameTransaction(t *testing.T) {
	r := newRepo(t)
	f := newFlight(t, "AR", "AR1550", block)
	if err := r.Create(ctx, f, event("vuelos.prueba")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	rows := outbox(t)
	if len(rows) != 1 || rows[0].Type != "vuelos.prueba" || rows[0].DataVersion != 1 || rows[0].Data != `{"flight_id": "x"}` {
		t.Errorf("outbox: %+v", rows)
	}

	// Si falla el vuelo (id repetido), tampoco queda el evento.
	if err := r.Create(ctx, f, event("vuelos.prueba")); err == nil {
		t.Fatal("se esperaba un error por id repetido")
	}
	if n := len(outbox(t)); n != 1 {
		t.Errorf("quedaron %d eventos, want 1", n)
	}
}

func TestUpdateSavesChangeVersionAndEvents(t *testing.T) {
	r := newRepo(t)
	f := newFlight(t, "AR", "AR1550", block)
	if err := r.Create(ctx, f); err != nil {
		t.Fatalf("Create: %v", err)
	}
	slot := domain.Slot{RunwayID: "05-23", BlockStart: block}
	updated, err := r.Update(ctx, f.ID, func(f *domain.Flight) ([]domain.Event, error) {
		return []domain.Event{event("vuelos.prueba")}, f.AssignSlot(slot, now)
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := r.Get(ctx, f.ID)
	if got.Status != domain.Scheduled || got.Slot == nil || *got.Slot != slot || got.Version != 2 || updated.Version != 2 {
		t.Errorf("el vuelo tiene que quedar programado con el slot y la versión 2: %+v", got)
	}
	if rows := outbox(t); len(rows) != 1 || rows[0].DataVersion != 2 {
		t.Errorf("el evento tiene que llevar la versión 2: %+v", rows)
	}
}

func TestUpdateSavesNothingIfTheChangeFails(t *testing.T) {
	r := newRepo(t)
	f := newFlight(t, "AR", "AR1550", block)
	if err := r.Create(ctx, f); err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err := r.Update(ctx, f.ID, func(f *domain.Flight) ([]domain.Event, error) {
		f.Status = domain.Cancelled // cambia en memoria y después falla
		return []domain.Event{event("vuelos.prueba")}, domain.ErrInvalidTransition
	})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("got %v, want ErrInvalidTransition", err)
	}
	got, _ := r.Get(ctx, f.ID)
	if got.Status != domain.PendingSlot || got.Version != 1 || len(outbox(t)) != 0 {
		t.Errorf("no se tendría que haber guardado nada: %+v", got)
	}
}

func TestUpdateNotFound(t *testing.T) {
	r := newRepo(t)
	_, err := r.Update(ctx, domain.NewID(), func(*domain.Flight) ([]domain.Event, error) { return nil, nil })
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// Diez pedidos a la vez quieren asignar un slot al mismo vuelo pendiente. Con la
// fila bloqueada, el primero lo programa y los demás ven que ya está programado.
// Sin el bloqueo, varios leerían "pendiente" a la vez y se pisarían.
func TestConcurrentUpdatesDoNotOverwriteEachOther(t *testing.T) {
	r := newRepo(t)
	f := newFlight(t, "AR", "AR1550", block)
	if err := r.Create(ctx, f); err != nil {
		t.Fatalf("Create: %v", err)
	}

	const workers = 10
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slot := domain.Slot{RunwayID: "05-23", BlockStart: block.Add(time.Duration(i) * domain.BlockDuration)}
			_, err := r.Update(ctx, f.ID, func(f *domain.Flight) ([]domain.Event, error) {
				return []domain.Event{event("vuelos.prueba")}, f.AssignSlot(slot, now)
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	ok := 0
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, domain.ErrInvalidTransition):
			t.Errorf("error inesperado: %v", err)
		}
	}
	got, _ := r.Get(ctx, f.ID)
	if ok != 1 || got.Version != 2 || len(outbox(t)) != 1 {
		t.Errorf("tenía que ganar uno solo: %d éxitos, versión %d, %d eventos", ok, got.Version, len(outbox(t)))
	}
}

func TestApplyMessageAppliesEachMessageOnce(t *testing.T) {
	r := newRepo(t)
	f := newFlight(t, "AR", "AR1550", block)
	if err := r.Create(ctx, f); err != nil {
		t.Fatalf("Create: %v", err)
	}
	messageID := domain.NewID()
	assign := func(f *domain.Flight) ([]domain.Event, error) {
		return nil, f.AssignSlot(domain.Slot{RunwayID: "05-23", BlockStart: block}, now)
	}

	applied, err := r.ApplyMessage(ctx, messageID, f.ID, assign)
	if err != nil || !applied {
		t.Fatalf("primera vez: applied %v, err %v", applied, err)
	}
	// RabbitMQ lo vuelve a entregar: no se aplica de nuevo.
	applied, err = r.ApplyMessage(ctx, messageID, f.ID, assign)
	if err != nil || applied {
		t.Fatalf("segunda vez: applied %v, err %v", applied, err)
	}
	if got, _ := r.Get(ctx, f.ID); got.Version != 2 {
		t.Errorf("el mensaje se aplicó más de una vez: versión %d", got.Version)
	}
}

func TestApplyMessageDoesNotMarkTheMessageIfTheChangeFails(t *testing.T) {
	r := newRepo(t)
	f := newFlight(t, "AR", "AR1550", block)
	if err := r.Create(ctx, f); err != nil {
		t.Fatalf("Create: %v", err)
	}
	messageID := domain.NewID()
	failing := func(*domain.Flight) ([]domain.Event, error) { return nil, errors.New("falla pasajera") }
	if _, err := r.ApplyMessage(ctx, messageID, f.ID, failing); err == nil {
		t.Fatal("se esperaba un error")
	}
	// Al reintentar, el mensaje se aplica, porque la falla no lo dejó registrado.
	applied, err := r.ApplyMessage(ctx, messageID, f.ID, func(f *domain.Flight) ([]domain.Event, error) {
		return nil, f.AssignSlot(domain.Slot{RunwayID: "05-23", BlockStart: block}, now)
	})
	if err != nil || !applied {
		t.Errorf("reintento: applied %v, err %v", applied, err)
	}
}

func TestListByAirline(t *testing.T) {
	r := newRepo(t)
	// 21 vuelos de AR, cada uno 15 minutos después del anterior, y uno de LA.
	var ar []*domain.Flight
	for i := range 21 {
		f := newFlight(t, "AR", fmt.Sprintf("AR%d", 1000+i), block.Add(time.Duration(i)*domain.BlockDuration))
		if err := r.Create(ctx, f); err != nil {
			t.Fatalf("Create: %v", err)
		}
		ar = append(ar, f)
	}
	if err := r.Create(ctx, newFlight(t, "LA", "LA4021", block)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	page1, total, err := r.ListByAirline(ctx, "AR", "", 1)
	if err != nil {
		t.Fatalf("ListByAirline: %v", err)
	}
	page2, _, _ := r.ListByAirline(ctx, "AR", "", 2)
	if total != 21 || len(page1) != PageSize || len(page2) != 1 {
		t.Fatalf("total %d, página 1 con %d, página 2 con %d", total, len(page1), len(page2))
	}
	for _, f := range append(page1, page2...) {
		if f.Airline != "AR" {
			t.Errorf("apareció un vuelo de otra aerolínea: %s", f.Airline)
		}
	}
	if page1[0].Number != "AR1000" {
		t.Errorf("tiene que empezar por el más temprano, empezó por %s", page1[0].Number)
	}

	// El primero pasa a tener un slot a la noche: con slot vale ese horario (RN-39).
	evening := domain.Slot{RunwayID: "05-23", BlockStart: block.Add(12 * time.Hour)}
	if _, err := r.Update(ctx, ar[0].ID, func(f *domain.Flight) ([]domain.Event, error) {
		return nil, f.AssignSlot(evening, now)
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	page2, _, _ = r.ListByAirline(ctx, "AR", "", 2)
	if len(page2) != 1 || page2[0].Number != "AR1000" {
		t.Errorf("con el slot de la noche tiene que quedar último: %+v", page2)
	}

	scheduled, total, _ := r.ListByAirline(ctx, "AR", domain.Scheduled, 1)
	if total != 1 || len(scheduled) != 1 || scheduled[0].Number != "AR1000" {
		t.Errorf("filtro por estado: total %d, %+v", total, scheduled)
	}
}

// createWithEvents guarda n vuelos, cada uno con un evento, un segundo después del anterior.
func createWithEvents(t *testing.T, r *Repository, n int) []string {
	t.Helper()
	var ids []string
	for i := range n {
		e := event("vuelos.prueba")
		e.OccurredAt = now.Add(time.Duration(i) * time.Second)
		e.TraceParent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
		if err := r.Create(ctx, newFlight(t, "AR", fmt.Sprintf("AR%d", 2000+i), block), e); err != nil {
			t.Fatalf("Create: %v", err)
		}
		ids = append(ids, e.ID)
	}
	return ids
}

func TestPublishPendingPublishesInOrderAndDeletes(t *testing.T) {
	r := newRepo(t)
	ids := createWithEvents(t, r, 3)

	var got []domain.Event
	n, err := r.PublishPending(ctx, func(_ context.Context, e domain.Event) error {
		got = append(got, e)
		return nil
	})
	if err != nil || n != 3 {
		t.Fatalf("publicados %d, error %v", n, err)
	}
	for i, e := range got {
		if e.ID != ids[i] {
			t.Errorf("tienen que salir en el orden en que se guardaron: %d es %s", i, e.ID)
		}
	}
	if got[0].TraceParent == "" || string(got[0].Data.(json.RawMessage)) != `{"flight_id": "x"}` {
		t.Errorf("el evento tiene que traer la traza y los datos: %+v", got[0])
	}
	if left := len(outbox(t)); left != 0 {
		t.Errorf("quedaron %d en el outbox", left)
	}
}

func TestPublishPendingStopsAtTheFirstFailure(t *testing.T) {
	r := newRepo(t)
	ids := createWithEvents(t, r, 3)

	n, err := r.PublishPending(ctx, func(_ context.Context, e domain.Event) error {
		if e.ID == ids[1] {
			return errors.New("RabbitMQ no responde")
		}
		return nil
	})
	if err == nil || n != 1 {
		t.Fatalf("publicados %d, error %v", n, err)
	}
	// El primero salió y se borró; el que falló y el siguiente quedan para la próxima.
	rows := outbox(t)
	if len(rows) != 2 || rows[0].ID != ids[1] || rows[1].ID != ids[2] {
		t.Errorf("outbox: %+v", rows)
	}
}

// Las dos instancias de Vuelos corren el relay a la vez (4.21): cada evento sale una sola vez.
func TestTwoRelaysDoNotPublishTheSameEvent(t *testing.T) {
	r := newRepo(t)
	createWithEvents(t, r, 30)

	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n, err := r.PublishPending(ctx, func(_ context.Context, e domain.Event) error {
					mu.Lock()
					seen[e.ID]++
					mu.Unlock()
					time.Sleep(5 * time.Millisecond) // como si esperara la confirmación de RabbitMQ
					return nil
				})
				if err != nil {
					t.Errorf("PublishPending: %v", err)
					return
				}
				if n == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()
	if len(seen) != 30 {
		t.Errorf("salieron %d eventos distintos, want 30", len(seen))
	}
	for id, times := range seen {
		if times != 1 {
			t.Errorf("el evento %s salió %d veces", id, times)
		}
	}
}
