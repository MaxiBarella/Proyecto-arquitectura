package repositories

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	"gorm.io/gorm"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/models"
)

// Un solo MySQL real para todos los tests, con la misma tabla que el Docker
// Compose (db/001_schema.sql). Con -short o sin Docker, los tests se saltean.
var (
	testDB    *gorm.DB
	skipCause string
)

func TestMain(m *testing.M) {
	flag.Parse()
	code, err := runWithMySQL(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runWithMySQL(m *testing.M) (int, error) {
	if testing.Short() {
		skipCause = "test de integración: se saltea con -short"
		return m.Run(), nil
	}
	ctx := context.Background()
	if err := dockerAvailable(); err != nil {
		skipCause = "Docker no está disponible: " + err.Error()
		return m.Run(), nil
	}
	container, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase("usuarios"),
		tcmysql.WithUsername("usuarios"),
		tcmysql.WithPassword("usuarios"),
		tcmysql.WithScripts("../../db/001_schema.sql"),
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

func newRepo(t *testing.T) *UserRepository {
	t.Helper()
	if testDB == nil {
		t.Skip(skipCause)
	}
	if err := testDB.Exec("DELETE FROM users").Error; err != nil {
		t.Fatal(err)
	}
	return NewUserRepository(testDB)
}

var (
	ctx = context.Background()
	now = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
)

func user(id, username, hash, role, airline string) models.User {
	return models.User{ID: id, Username: username, PasswordHash: hash, Role: role, Airline: airline, CreatedAt: now, UpdatedAt: now}
}

// hash60 arma un texto de 60 caracteres, el largo de un hash de bcrypt.
func hash60(c byte) string {
	b := make([]byte, 60)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func TestUpsertYFindByUsername(t *testing.T) {
	r := newRepo(t)
	if err := r.Upsert(ctx, user("id-1", "aerolineas", hash60('a'), models.RoleAirline, "AR")); err != nil {
		t.Fatal(err)
	}
	got, err := r.FindByUsername(ctx, "aerolineas")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "id-1" || got.Role != models.RoleAirline || got.Airline != "AR" || got.PasswordHash != hash60('a') {
		t.Fatalf("usuario inesperado: %+v", got)
	}
}

func TestUpsertOperadorSinAerolinea(t *testing.T) {
	r := newRepo(t)
	if err := r.Upsert(ctx, user("id-1", "operador", hash60('o'), models.RoleOperator, "")); err != nil {
		t.Fatal(err)
	}
	got, err := r.FindByUsername(ctx, "operador")
	if err != nil {
		t.Fatal(err)
	}
	if got.Airline != "" {
		t.Fatalf("el operador quedó con aerolínea %q", got.Airline)
	}
}

func TestUpsertMismoNombreActualizaSinCambiarElID(t *testing.T) {
	r := newRepo(t)
	if err := r.Upsert(ctx, user("id-1", "latam", hash60('a'), models.RoleAirline, "LA")); err != nil {
		t.Fatal(err)
	}
	if err := r.Upsert(ctx, user("id-2", "latam", hash60('b'), models.RoleAirline, "LA")); err != nil {
		t.Fatal(err)
	}
	got, err := r.FindByUsername(ctx, "latam")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "id-1" || got.PasswordHash != hash60('b') {
		t.Fatalf("se esperaba el id-1 con la contraseña nueva: %+v", got)
	}
}

func TestFindByUsernameInexistente(t *testing.T) {
	r := newRepo(t)
	if _, err := r.FindByUsername(ctx, "nadie"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error %v, se esperaba ErrNotFound", err)
	}
}

func TestLaBaseRechazaAerolineaSinCodigo(t *testing.T) {
	r := newRepo(t)
	if err := r.Upsert(ctx, user("id-1", "rota", hash60('a'), models.RoleAirline, "")); err == nil {
		t.Fatal("una aerolínea sin código tiene que rechazarse (chk_users_airline)")
	}
}
