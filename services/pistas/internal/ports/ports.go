package ports

import (
	"context"
	"errors"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

// ErrConflicto indica que la operación chocó con otra simultánea (dos pedidos por el mismo slot o
// por el mismo vuelo). Se puede reintentar: la segunda vez ya se ve lo que guardó la otra.
var ErrConflicto = errors.New("conflicto con una operación simultánea")

// Repositorio es el puerto de salida hacia la base de Pistas.
type Repositorio interface {
	// EnTransaccion corre fn dentro de una transacción. Si fn devuelve error, se deshace todo.
	EnTransaccion(ctx context.Context, fn func(ctx context.Context, tx Transaccion) error) error
	Lectura
}

// Lectura son las consultas que no necesitan transacción.
type Lectura interface {
	Pistas(ctx context.Context) ([]domain.Pista, error)
	// SlotsOcupados devuelve los slots con inicio en [desde, hasta).
	SlotsOcupados(ctx context.Context, desde, hasta time.Time) ([]domain.Slot, error)
}

// Transaccion son las operaciones que se hacen juntas al asignar un slot.
type Transaccion interface {
	Lectura
	// Pista devuelve domain.ErrPistaInexistente si no existe. La lee con un bloqueo compartido:
	// varios pedidos de slot la pueden leer a la vez, pero un cierre espera a que terminen, y un
	// pedido espera a que termine un cierre (ADR-004).
	Pista(ctx context.Context, id string) (domain.Pista, error)
	// PistasConBloqueo devuelve todas las pistas y las bloquea hasta el final de la transacción,
	// para cerrarlas o reabrirlas sin que se cruce un pedido de slot.
	PistasConBloqueo(ctx context.Context) ([]domain.Pista, error)
	// GuardarPista guarda si está abierta y quién la cerró.
	GuardarPista(ctx context.Context, p domain.Pista) error
	// SlotsDePista devuelve los slots de la pista, ordenados por bloque.
	SlotsDePista(ctx context.Context, pistaID string) ([]domain.Slot, error)
	// Alternativos devuelve los aeropuertos alternativos ordenados por distancia (RN-25).
	Alternativos(ctx context.Context) ([]domain.Alternativo, error)
	// Clima devuelve lo último que Pistas sabe del clima, y lo bloquea hasta el final de la
	// transacción, así dos eventos de clima se aplican de a uno.
	Clima(ctx context.Context) (domain.EstadoClima, error)
	GuardarClima(ctx context.Context, c domain.EstadoClima) error
	// SlotDelVuelo devuelve nil si el vuelo no tiene slot.
	SlotDelVuelo(ctx context.Context, vueloID string) (*domain.Slot, error)
	// OcupanteConBloqueo devuelve quién ocupa el slot, o nil, y lo bloquea hasta el final de la
	// transacción para que nadie lo cambie mientras se decide.
	OcupanteConBloqueo(ctx context.Context, pistaID string, inicio time.Time) (*domain.Slot, error)
	// Ocupar guarda el slot y le completa el ID. Devuelve ErrConflicto si el slot o el vuelo ya
	// están tomados por otra transacción.
	Ocupar(ctx context.Context, s *domain.Slot) error
	Liberar(ctx context.Context, slotID int64) error
	// GuardarEvento lo agrega al outbox, en la misma transacción (4.4).
	GuardarEvento(ctx context.Context, e domain.Evento) error
	// MarcarProcesado registra que el mensaje se aplicó. Devuelve false si ya estaba registrado:
	// el mensaje llegó dos veces y no hay que aplicarlo de nuevo (4.9).
	MarcarProcesado(ctx context.Context, mensajeID string) (bool, error)
}

// EventoGuardado es un evento del outbox, con el número de orden que le dio la base.
type EventoGuardado struct {
	domain.Evento
	Secuencia int64
	// DatosJSON son los datos tal como se guardaron.
	DatosJSON []byte
}

// Outbox es el puerto de salida que usa el relay para leer lo pendiente de publicar.
type Outbox interface {
	Pendientes(ctx context.Context, limite int) ([]EventoGuardado, error)
	MarcarPublicado(ctx context.Context, secuencia int64) error
}

// Publicador es el puerto de salida hacia la mensajería.
type Publicador interface {
	Publicar(ctx context.Context, e EventoGuardado) error
}
