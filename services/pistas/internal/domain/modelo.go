package domain

import "time"

// DuracionBloque es lo que dura un bloque de pista (pendiente 1 de la sección 18 de decisiones.md).
const DuracionBloque = 15 * time.Minute

// ZonaCordoba es la hora de Córdoba. Argentina no cambia la hora en verano, así que alcanza con
// un desplazamiento fijo y la imagen del contenedor no necesita la base de zonas horarias.
var ZonaCordoba = time.FixedZone("ART", -3*60*60)

// TipoOperacion dice si un vuelo aterriza o despega en nuestro aeropuerto.
type TipoOperacion string

const (
	Arribo   TipoOperacion = "arribo"
	Despegue TipoOperacion = "despegue"
)

// Valido indica si el tipo es uno de los dos que existen.
func (t TipoOperacion) Valido() bool {
	return t == Arribo || t == Despegue
}

// Pista es el recurso físico donde se aterriza y se despega.
type Pista struct {
	ID      int64
	Nombre  string
	Abierta bool
}

// Slot es una pista más un bloque, ocupado por un vuelo. Un bloque sin vuelo no se guarda: está libre.
type Slot struct {
	ID            int64
	PistaID       int64
	InicioBloque  time.Time
	VueloID       string
	TipoOperacion TipoOperacion
}

// MismoLugar indica si el slot está en esa pista y ese bloque.
func (s Slot) MismoLugar(pistaID int64, inicio time.Time) bool {
	return s.PistaID == pistaID && s.InicioBloque.Equal(inicio)
}

// PedidoDeSlot es lo que pide Vuelos para un vuelo: una pista y un bloque exactos (RN-07).
type PedidoDeSlot struct {
	VueloID       string
	TipoOperacion TipoOperacion
	PistaID       int64
	InicioBloque  time.Time
}

// SlotLibre es un lugar que se puede pedir.
type SlotLibre struct {
	PistaID      int64
	InicioBloque time.Time
}

// Resultado es lo que devuelve una asignación exitosa.
type Resultado struct {
	Slot Slot
	// YaAsignado es verdadero cuando el pedido se repitió y el vuelo ya tenía este slot (4.6).
	YaAsignado bool
	// VueloDesplazadoID es el despegue que perdió el slot ante un arribo (RN-11), si hubo uno.
	VueloDesplazadoID string
}

// Tipos de evento que publica Pistas (docs/contracts/pistas-eventos.md).
const (
	EventoSlotAsignado = "slot.asignado"
	EventoSlotCedido   = "slot.cedido"
)

// Evento es un evento de dominio que se guarda en el outbox y se publica después (4.4).
type Evento struct {
	ID            string
	Tipo          string
	Fecha         time.Time
	CorrelacionID string
	Datos         any
}

// DatosSlotAsignado son los datos de "slot.asignado".
type DatosSlotAsignado struct {
	VueloID       string        `json:"vuelo_id"`
	SlotID        int64         `json:"slot_id"`
	PistaID       int64         `json:"pista_id"`
	InicioBloque  time.Time     `json:"inicio_bloque"`
	TipoOperacion TipoOperacion `json:"tipo_operacion"`
}

// DatosSlotCedido son los datos de "slot.cedido". VueloID es el despegue que perdió el slot.
type DatosSlotCedido struct {
	VueloID        string    `json:"vuelo_id"`
	SlotID         int64     `json:"slot_id"`
	PistaID        int64     `json:"pista_id"`
	InicioBloque   time.Time `json:"inicio_bloque"`
	CedidoAVueloID string    `json:"cedido_a_vuelo_id"`
}
