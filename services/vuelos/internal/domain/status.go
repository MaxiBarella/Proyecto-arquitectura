package domain

// FlightType es el tipo de operación. No cambia (RN-20).
type FlightType string

const (
	Arrival   FlightType = "arrival"
	Departure FlightType = "departure"
)

// Scope es el ámbito del vuelo.
type Scope string

const (
	Domestic      Scope = "domestic"
	International Scope = "international"
)

// Status es el estado del vuelo (sección 6 del SPEC).
type Status string

const (
	PendingSlot Status = "pending_slot"
	Scheduled   Status = "scheduled"
	Delayed     Status = "delayed"
	Completed   Status = "completed"
	Diverted    Status = "diverted"
	Cancelled   Status = "cancelled"
)

// IsFinal dice si el estado es final: de ahí no se sale (RN-21).
func (s Status) IsFinal() bool {
	return s == Completed || s == Diverted || s == Cancelled
}

// transitions es la tabla de transiciones válidas de la sección 6 del SPEC.
// Para cada estado de origen y de destino, dice a qué tipos de vuelo se aplica.
// Cualquier par que no esté acá se rechaza (RN-21).
var transitions = map[Status]map[Status][]FlightType{
	PendingSlot: {
		Scheduled: {Arrival, Departure},
		Cancelled: {Arrival, Departure},
	},
	Scheduled: {
		Delayed:   {Departure},
		Diverted:  {Arrival},
		Completed: {Arrival, Departure},
		Cancelled: {Arrival, Departure},
	},
	Delayed: {
		Scheduled: {Departure},
		Cancelled: {Arrival, Departure},
	},
}

// CanTransition dice si un vuelo del tipo dado puede pasar de un estado a otro.
func CanTransition(from, to Status, t FlightType) bool {
	for _, allowed := range transitions[from][to] {
		if allowed == t {
			return true
		}
	}
	return false
}
