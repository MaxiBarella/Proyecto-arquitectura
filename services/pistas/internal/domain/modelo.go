package domain

import "time"

// DuracionBloque es lo que dura un bloque de pista (RN-03).
const DuracionBloque = 15 * time.Minute

// TipoOperacion dice si un vuelo aterriza o despega en nuestro aeropuerto. Los valores son los del
// contrato con Vuelos (services/pistas/openapi.yaml).
type TipoOperacion string

const (
	Arribo   TipoOperacion = "arrival"
	Despegue TipoOperacion = "departure"
)

// Valido indica si el tipo es uno de los dos que existen.
func (t TipoOperacion) Valido() bool {
	return t == Arribo || t == Despegue
}

// MotivoCierre dice quién cerró una pista. Los valores son los del evento de pista cerrada (4.12).
type MotivoCierre string

const (
	CerradaPorClima    MotivoCierre = "weather"
	CerradaPorOperador MotivoCierre = "operator"
)

// Pista es el recurso físico donde se aterriza y se despega. Su ID es el nombre, sin barra, como
// "05-23" (4.11).
type Pista struct {
	ID      string
	Abierta bool
	// CerradaPor está vacío si la pista está abierta.
	CerradaPor MotivoCierre
}

// NuestroAeropuerto es el único que maneja el sistema (2.8). Los eventos de clima de otro se ignoran.
const NuestroAeropuerto = "SACO"

// EstadoClima es lo último que Pistas sabe del clima, por los eventos de Clima (4.2).
type EstadoClima struct {
	// Operable es nil si todavía no llegó ningún dato.
	Operable *bool
	// ConDatos es falso después de "Clima sin datos": las pistas no cambian solas (RN-24).
	ConDatos bool
}

// Alternativo es un aeropuerto al que se puede desviar un arribo (RN-25).
type Alternativo struct {
	ICAO       string
	Nombre     string
	Disponible bool
}

// Slot es una pista más un bloque, ocupado por un vuelo. Un bloque sin vuelo no se guarda: está libre.
type Slot struct {
	ID            int64
	PistaID       string
	InicioBloque  time.Time
	VueloID       string
	TipoOperacion TipoOperacion
}

// MismoLugar indica si el slot está en esa pista y ese bloque.
func (s Slot) MismoLugar(pistaID string, inicio time.Time) bool {
	return s.PistaID == pistaID && s.InicioBloque.Equal(inicio)
}

// PedidoDeSlot es lo que pide Vuelos para un vuelo: una pista y un bloque exactos (RN-07).
type PedidoDeSlot struct {
	VueloID       string
	TipoOperacion TipoOperacion
	PistaID       string
	InicioBloque  time.Time
}

// SlotLibre es un lugar que se puede pedir.
type SlotLibre struct {
	PistaID      string
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

// Claves de ruteo de los eventos que publica Pistas (4.14).
const (
	EventoSlotAsignado   = "pistas.slot.asignado"
	EventoSlotCedido     = "pistas.slot.cedido"
	EventoSlotLiberado   = "pistas.slot.liberado_por_cierre"
	EventoPistaCerrada   = "pistas.pista.cerrada"
	EventoPistaReabierta = "pistas.pista.reabierta"
)

// Evento es un evento de dominio que se guarda en el outbox y se publica después (4.4).
type Evento struct {
	ID            string
	Tipo          string
	Fecha         time.Time
	CorrelacionID string
	// TraceParent es la traza del pedido que lo causó, para que siga en el mensaje (11.6).
	TraceParent string
	Datos       any
}

// DatosSlotAsignado son los datos de "slot asignado" (4.12).
type DatosSlotAsignado struct {
	VueloID      string    `json:"flight_id"`
	PistaID      string    `json:"runway_id"`
	InicioBloque time.Time `json:"block_start"`
}

// DatosSlotCedido son los datos de "slot cedido a un arribo" (4.12). VueloID es el despegue que
// perdió el slot y CedidoAVueloID, el arribo que se lo queda.
type DatosSlotCedido struct {
	VueloID        string    `json:"flight_id"`
	PistaID        string    `json:"runway_id"`
	InicioBloque   time.Time `json:"block_start"`
	CedidoAVueloID string    `json:"taken_by_flight_id"`
}

// DatosSlotLiberado son los datos de "slot liberado por cierre" (4.12). DesviadoA es el alternativo
// de un arribo (RN-25), o nil si es un despegue, que queda demorado (RN-17).
type DatosSlotLiberado struct {
	VueloID      string    `json:"flight_id"`
	PistaID      string    `json:"runway_id"`
	InicioBloque time.Time `json:"block_start"`
	DesviadoA    *string   `json:"diverted_to"`
}

// DatosPista son los datos de "pista cerrada" y "pista reabierta" (4.12): la pista y quién la cerró.
type DatosPista struct {
	PistaID string       `json:"runway_id"`
	Motivo  MotivoCierre `json:"reason"`
}
