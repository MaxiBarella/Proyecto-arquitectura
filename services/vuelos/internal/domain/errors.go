package domain

import "errors"

// Errores del dominio. Las capas de afuera los traducen a los códigos del contrato
// (openapi.yaml): ErrInvalidFlight a INVALID_REQUEST, ErrInvalidTransition a
// INVALID_TRANSITION, ErrNotOwner a NOT_FLIGHT_OWNER y ErrNotFound a FLIGHT_NOT_FOUND.
var (
	ErrInvalidFlight     = errors.New("datos del vuelo inválidos")
	ErrInvalidTransition = errors.New("transición de estado no permitida")
	ErrNotOwner          = errors.New("el vuelo es de otra aerolínea")
	ErrNotFound          = errors.New("vuelo no encontrado")
)
