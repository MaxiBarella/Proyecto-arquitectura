package mysql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Los primeros tests no usan la base: prueban la regla de reintento sola.

func deadlock() error {
	return fmt.Errorf("guardar el evento en el outbox: %w", &driver.MySQLError{Number: errDeadlock, Message: "Deadlock found"})
}

func TestRetryOnConflictRetriesDeadlockUntilItWorks(t *testing.T) {
	calls := 0
	err := retryOnConflict(context.Background(), func() error {
		calls++
		if calls < conflictAttempts {
			return deadlock()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("err = %v, se esperaba nil", err)
	}
	if calls != conflictAttempts {
		t.Fatalf("se corrió %d veces, se esperaban %d", calls, conflictAttempts)
	}
}

func TestRetryOnConflictRetriesLockWaitTimeout(t *testing.T) {
	calls := 0
	err := retryOnConflict(context.Background(), func() error {
		calls++
		if calls == 1 {
			return &driver.MySQLError{Number: errLockWaitTimeout, Message: "Lock wait timeout exceeded"}
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("err = %v, llamadas = %d; se esperaba nil y 2", err, calls)
	}
}

func TestRetryOnConflictGivesUpAfterTheLastAttempt(t *testing.T) {
	calls := 0
	err := retryOnConflict(context.Background(), func() error {
		calls++
		return deadlock()
	})
	if !isConflict(err) {
		t.Fatalf("err = %v, se esperaba el deadlock", err)
	}
	if calls != conflictAttempts {
		t.Fatalf("se corrió %d veces, se esperaban %d", calls, conflictAttempts)
	}
}

func TestRetryOnConflictDoesNotRetryOtherErrors(t *testing.T) {
	other := errors.New("MySQL no responde")
	for _, err := range []error{other, &driver.MySQLError{Number: 1062, Message: "Duplicate entry"}} {
		calls := 0
		got := retryOnConflict(context.Background(), func() error {
			calls++
			return err
		})
		if !errors.Is(got, err) || calls != 1 {
			t.Fatalf("con %v: err = %v, llamadas = %d; se esperaba el mismo error y 1", err, got, calls)
		}
	}
}

func TestRetryOnConflictStopsWhenTheRequestIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := retryOnConflict(ctx, func() error {
		calls++
		cancel()
		return deadlock()
	})
	if !isConflict(err) || calls != 1 {
		t.Fatalf("err = %v, llamadas = %d; se esperaba el deadlock y 1", err, calls)
	}
}

// Con un MySQL real: dos transacciones toman dos vuelos en orden cruzado y MySQL corta una
// por interbloqueo. La que corta es la del repositorio, porque la otra ya modificó más filas
// (MySQL descarta la que menos trabajo hizo). El repositorio la vuelve a correr y termina bien.
func TestTransactionRetriesARealDeadlock(t *testing.T) {
	r := newRepo(t)
	var ids []string
	for i := 0; i < 4; i++ {
		f := newFlight(t, "AR", fmt.Sprintf("AR%04d", i), block)
		if err := r.Create(ctx, f); err != nil {
			t.Fatalf("Create: %v", err)
		}
		ids = append(ids, f.ID)
	}
	a, b := ids[0], ids[1]

	// La otra transacción modifica b y dos vuelos más: queda con más peso.
	other := testDB.Begin()
	defer other.Rollback()
	if err := other.Exec("UPDATE flights SET version = version + 1 WHERE id IN ?", ids[1:]).Error; err != nil {
		t.Fatalf("la otra transacción no pudo modificar: %v", err)
	}

	aLocked := make(chan struct{}, 1)
	calls := 0
	done := make(chan error, 1)
	go func() {
		done <- r.transaction(ctx, func(tx *gorm.DB) error {
			calls++
			var rowA, rowB flightRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rowA, "id = ?", a).Error; err != nil {
				return err
			}
			if calls == 1 {
				aLocked <- struct{}{}
			}
			// Espera a b, que tiene la otra transacción.
			return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rowB, "id = ?", b).Error
		})
	}()

	<-aLocked
	time.Sleep(300 * time.Millisecond) // que el repositorio quede esperando a b
	// La otra pide a: se cierra el ciclo y MySQL corta una de las dos.
	var row flightRow
	if err := other.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", a).Error; err != nil {
		t.Fatalf("MySQL cortó la otra transacción en vez de la del repositorio: %v", err)
	}
	if err := other.Commit().Error; err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("la transacción del repositorio terminó con error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("la transacción se corrió %d veces, se esperaban 2 (la cortada y el reintento)", calls)
	}
}
