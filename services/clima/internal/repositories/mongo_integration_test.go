package repositories

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

// Tests de integración contra un MongoDB real, que levanta testcontainers-go (20.1).
// Necesitan Docker; si no está, se saltean. Con -short también se saltean.

func newTestDB(t *testing.T) *mongo.Database {
	t.Helper()
	if testing.Short() {
		t.Skip("test de integración: se saltea con -short")
	}
	ctx := context.Background()
	container, err := tcmongo.Run(ctx, "mongo:7")
	if err != nil {
		t.Skipf("no se pudo levantar MongoDB con Docker: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client, db, err := Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return db
}

func event(id string, version int64) models.PendingEvent {
	return models.PendingEvent{ID: id, Type: models.EventAptitudeChanged, DataVersion: version, OccurredAt: time.Now().UTC()}
}

func TestSavePublishedContraMongoDB(t *testing.T) {
	db := newTestDB(t)
	repo := NewAptitudeRepository(db)
	ctx := context.Background()
	state := models.PublishedState{Operable: true, Reasons: []string{}, ObservedAt: time.Now().UTC().Truncate(time.Millisecond)}

	t.Run("crea el documento si no existe", func(t *testing.T) {
		if err := repo.SavePublished(ctx, "SACO", 0, state, []models.PendingEvent{event("e1", 1)}); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Get(ctx, "SACO")
		if err != nil {
			t.Fatal(err)
		}
		if got.DataVersion != 1 || len(got.Outbox) != 1 || got.Published == nil || !got.Published.Operable {
			t.Errorf("documento inesperado: %+v", got)
		}
	})

	t.Run("rechaza una versión vieja", func(t *testing.T) {
		err := repo.SavePublished(ctx, "SACO", 0, state, []models.PendingEvent{event("e2", 1)})
		if !errors.Is(err, ErrConflict) {
			t.Errorf("con la versión 0 sobre un documento en la 1 se esperaba ErrConflict, fue %v", err)
		}
	})

	t.Run("acepta la versión actual", func(t *testing.T) {
		if err := repo.SavePublished(ctx, "SACO", 1, state, []models.PendingEvent{event("e2", 2)}); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.Get(ctx, "SACO")
		if got.DataVersion != 2 || len(got.Outbox) != 2 {
			t.Errorf("documento inesperado: %+v", got)
		}
	})

	t.Run("el relay saca del outbox lo publicado", func(t *testing.T) {
		pending, err := repo.PendingEvents(ctx)
		if err != nil || len(pending) != 1 || len(pending[0].Outbox) != 2 {
			t.Fatalf("pendientes = %+v, %v", pending, err)
		}
		if err := repo.MarkPublished(ctx, "SACO", "e1"); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.Get(ctx, "SACO")
		if len(got.Outbox) != 1 || got.Outbox[0].ID != "e2" {
			t.Errorf("outbox = %+v", got.Outbox)
		}
	})
}

func TestSavePublishedSobreUnDocumentoSoloConForzado(t *testing.T) {
	db := newTestDB(t)
	repo := NewAptitudeRepository(db)
	ctx := context.Background()

	// El operador fuerza antes de que haya ningún estado publicado: el documento
	// existe, pero sin data_version.
	forced := &models.ForcedCondition{Conditions: models.Conditions{Thunderstorm: true}, SetAt: time.Now().UTC()}
	if err := repo.SetForced(ctx, "SACO", forced); err != nil {
		t.Fatal(err)
	}
	state := models.PublishedState{Operable: false, Reasons: []string{models.ReasonThunderstorm}, Forced: true}
	if err := repo.SavePublished(ctx, "SACO", 0, state, []models.PendingEvent{event("e1", 1)}); err != nil {
		t.Fatalf("la versión 0 tendría que valer para un documento sin data_version: %v", err)
	}
	got, _ := repo.Get(ctx, "SACO")
	if got.Forced == nil || got.DataVersion != 1 {
		t.Errorf("tendría que conservar el forzado y quedar en la versión 1: %+v", got)
	}
}

func TestSavePublishedConcurrente(t *testing.T) {
	db := newTestDB(t)
	repo := NewAptitudeRepository(db)
	ctx := context.Background()
	state := models.PublishedState{Operable: true, Reasons: []string{}}

	// Diez procesos leen la versión 0 y guardan a la vez: solo uno puede ganar.
	const writers = 10
	var wg sync.WaitGroup
	results := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- repo.SavePublished(ctx, "SACO", 0, state, []models.PendingEvent{event(string(rune('a'+i)), 1)})
		}(i)
	}
	wg.Wait()
	close(results)

	ok, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Errorf("error inesperado: %v", err)
		}
	}
	if ok != 1 || conflicts != writers-1 {
		t.Errorf("ganó %d y hubo %d conflictos; se esperaba 1 y %d", ok, conflicts, writers-1)
	}
	got, _ := repo.Get(ctx, "SACO")
	if got.DataVersion != 1 || len(got.Outbox) != 1 {
		t.Errorf("tendría que haber un solo evento en la versión 1: %+v", got)
	}
}
