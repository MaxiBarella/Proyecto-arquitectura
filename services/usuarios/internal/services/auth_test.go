package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/repositories"
)

// fakeStore guarda los usuarios en memoria, por nombre.
type fakeStore struct {
	users map[string]models.User
	err   error
}

func (f *fakeStore) FindByUsername(_ context.Context, username string) (models.User, error) {
	if f.err != nil {
		return models.User{}, f.err
	}
	u, ok := f.users[username]
	if !ok {
		return models.User{}, repositories.ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) Upsert(_ context.Context, u models.User) error {
	if old, ok := f.users[u.Username]; ok {
		u.ID = old.ID
	}
	f.users[u.Username] = u
	return nil
}

var (
	ctx    = context.Background()
	now    = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	secret = []byte("clave-de-prueba")
)

func newAuth(t *testing.T) (*AuthService, *fakeStore) {
	t.Helper()
	store := &fakeStore{users: map[string]models.User{}}
	s := NewAuthService(store, secret)
	s.now = func() time.Time { return now }
	err := s.LoadAccounts(ctx, []Account{
		{Username: "operador", Password: "op-123", Role: models.RoleOperator},
		{Username: "aerolineas", Password: "ar-123", Role: models.RoleAirline, Airline: "AR"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, store
}

func parse(t *testing.T, token string) jwt.MapClaims {
	t.Helper()
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(tok *jwt.Token) (any, error) {
		if tok.Method != jwt.SigningMethodHS256 {
			t.Fatalf("firma %v, se esperaba HS256", tok.Method)
		}
		return secret, nil
	}, jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("el token no verifica: %v", err)
	}
	return claims
}

func TestLoadAccountsHasheaLaContraseña(t *testing.T) {
	_, store := newAuth(t)
	u := store.users["operador"]
	if u.PasswordHash == "" || u.PasswordHash == "op-123" {
		t.Fatalf("la contraseña no quedó hasheada: %q", u.PasswordHash)
	}
	if len(u.PasswordHash) != 60 {
		t.Fatalf("un hash de bcrypt mide 60 caracteres, este mide %d", len(u.PasswordHash))
	}
}

func TestLoginAerolinea(t *testing.T) {
	s, store := newAuth(t)
	session, err := s.Login(ctx, "aerolineas", "ar-123")
	if err != nil {
		t.Fatal(err)
	}
	if session.Role != models.RoleAirline || session.Airline != "AR" || session.ExpiresIn != 3600 {
		t.Fatalf("sesión inesperada: %+v", session)
	}
	claims := parse(t, session.AccessToken)
	if claims["sub"] != store.users["aerolineas"].ID || claims["role"] != "airline" || claims["airline"] != "AR" {
		t.Fatalf("datos del token inesperados: %v", claims)
	}
	if claims["iat"] != float64(now.Unix()) || claims["exp"] != float64(now.Add(time.Hour).Unix()) {
		t.Fatalf("iat o exp inesperados: %v", claims)
	}
}

func TestLoginOperadorNoLlevaAerolinea(t *testing.T) {
	s, _ := newAuth(t)
	session, err := s.Login(ctx, "operador", "op-123")
	if err != nil {
		t.Fatal(err)
	}
	if session.Airline != "" {
		t.Fatalf("el operador no tiene aerolínea, vino %q", session.Airline)
	}
	if _, ok := parse(t, session.AccessToken)["airline"]; ok {
		t.Fatal("el token del operador no tiene que llevar airline (15.13)")
	}
}

func TestLoginMismoErrorSiFallaElUsuarioOLaContraseña(t *testing.T) {
	s, _ := newAuth(t)
	for _, c := range []struct{ user, pass string }{
		{"aerolineas", "otra"},
		{"nadie", "ar-123"},
	} {
		if _, err := s.Login(ctx, c.user, c.pass); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s/%s: error %v, se esperaba ErrInvalidCredentials", c.user, c.pass, err)
		}
	}
}

func TestLoadAccountsActualizaLaContraseña(t *testing.T) {
	s, store := newAuth(t)
	id := store.users["aerolineas"].ID
	err := s.LoadAccounts(ctx, []Account{{Username: "aerolineas", Password: "nueva", Role: models.RoleAirline, Airline: "AR"}})
	if err != nil {
		t.Fatal(err)
	}
	if store.users["aerolineas"].ID != id {
		t.Fatal("al recargar la cuenta cambió su identificador")
	}
	if _, err := s.Login(ctx, "aerolineas", "ar-123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("la contraseña vieja sigue andando: %v", err)
	}
	if _, err := s.Login(ctx, "aerolineas", "nueva"); err != nil {
		t.Fatalf("la contraseña nueva no anda: %v", err)
	}
}

func TestLoginErrorDeLaBase(t *testing.T) {
	s, store := newAuth(t)
	store.err = errors.New("MySQL no responde")
	if _, err := s.Login(ctx, "operador", "op-123"); err == nil || errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("un error de la base no es un error de credenciales: %v", err)
	}
}
