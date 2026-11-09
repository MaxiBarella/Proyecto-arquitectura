package models

import "time"

// Roles de un usuario (15.9). Son los valores que viajan en el token y en X-User-Role.
const (
	RoleAirline  = "airline"
	RoleOperator = "operator"
)

// User es una cuenta que puede iniciar sesión.
type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         string
	// Airline es el código de la aerolínea; vacío si el rol es operator.
	Airline   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Session es lo que devuelve un inicio de sesión correcto (15.11).
type Session struct {
	AccessToken string
	ExpiresIn   int
	Role        string
	Airline     string
}
