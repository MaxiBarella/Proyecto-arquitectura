package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/repositories"
)

// TokenTTL es lo que dura un token (15.2).
const TokenTTL = time.Hour

// ErrInvalidCredentials es el mismo error si el usuario no existe o si la
// contraseña no coincide, para no revelar qué usuarios existen (15.12).
var ErrInvalidCredentials = errors.New("usuario o contraseña incorrectos")

// UserStore es lo que el servicio necesita de la base.
type UserStore interface {
	FindByUsername(ctx context.Context, username string) (models.User, error)
	Upsert(ctx context.Context, u models.User) error
}

// Account es una cuenta de prueba que se carga al arrancar (15.15).
type Account struct {
	Username string
	Password string
	Role     string
	Airline  string
}

// AuthService verifica credenciales y entrega tokens.
type AuthService struct {
	users  UserStore
	secret []byte
	now    func() time.Time
	// dummyHash se compara cuando el usuario no existe, para que la respuesta tarde
	// lo mismo que con una contraseña equivocada y no delate qué usuarios existen.
	dummyHash []byte
}

func NewAuthService(users UserStore, secret []byte) *AuthService {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("sin-usuario"), bcrypt.DefaultCost)
	return &AuthService{users: users, secret: secret, now: time.Now, dummyHash: dummy}
}

// LoadAccounts guarda las cuentas de prueba con la contraseña hasheada con bcrypt (15.14).
func (s *AuthService) LoadAccounts(ctx context.Context, accounts []Account) error {
	for _, a := range accounts {
		hash, err := bcrypt.GenerateFromPassword([]byte(a.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hashear la contraseña de %s: %w", a.Username, err)
		}
		now := s.now().UTC()
		err = s.users.Upsert(ctx, models.User{
			ID:           newID(),
			Username:     a.Username,
			PasswordHash: string(hash),
			Role:         a.Role,
			Airline:      a.Airline,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Login verifica el usuario y la contraseña y entrega un token firmado.
func (s *AuthService) Login(ctx context.Context, username, password string) (models.Session, error) {
	user, err := s.users.FindByUsername(ctx, username)
	if errors.Is(err, repositories.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return models.Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return models.Session{}, ErrInvalidCredentials
	}

	token, err := s.sign(user)
	if err != nil {
		return models.Session{}, err
	}
	return models.Session{
		AccessToken: token,
		ExpiresIn:   int(TokenTTL.Seconds()),
		Role:        user.Role,
		Airline:     user.Airline,
	}, nil
}

// sign arma el JWT con HS256 (15.5) y los datos de la 15.13.
func (s *AuthService) sign(user models.User) (string, error) {
	now := s.now()
	claims := jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role,
		"iat":  now.Unix(),
		"exp":  now.Add(TokenTTL).Unix(),
	}
	if user.Role == models.RoleAirline {
		claims["airline"] = user.Airline
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("firmar el token: %w", err)
	}
	return token, nil
}

// newID genera un identificador aleatorio con forma de UUID versión 4, como en
// Clima y Vuelos (5.8).
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
