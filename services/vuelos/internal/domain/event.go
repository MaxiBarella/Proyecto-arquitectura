package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Event es un evento que publica Vuelos, guardado en el outbox junto con el cambio
// que lo produjo (4.4, 4.7). Tiene los campos del mensaje de la 4.8.
type Event struct {
	ID            string
	Type          string
	OccurredAt    time.Time
	CorrelationID string
	// DataVersion es la versión del vuelo después del cambio. La completa el
	// repositorio al guardar.
	DataVersion int64
	Data        any
}

// NewID genera un identificador aleatorio con forma de UUID versión 4. Es la misma
// función que usa Clima, para no sumar una librería.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
