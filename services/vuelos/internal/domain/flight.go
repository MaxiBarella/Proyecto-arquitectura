package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// HomeCity es la ciudad de nuestro aeropuerto (RN-01). Un despegue sale de acá y un
// arribo llega acá.
const HomeCity = "Córdoba"

// BlockDuration es la duración de un bloque de pista (RN-03).
const BlockDuration = 15 * time.Minute

var flightNumberPattern = regexp.MustCompile(`^[A-Z0-9]{2}[0-9]{1,4}$`)

var icaoPattern = regexp.MustCompile(`^[A-Z]{4}$`)

// Slot es una pista más un bloque (RN-04).
type Slot struct {
	RunwayID   string
	BlockStart time.Time
}

// End es el momento en que termina el bloque.
func (s Slot) End() time.Time {
	return s.BlockStart.Add(BlockDuration)
}

// ValidateSlot revisa el formato de un slot pedido: que tenga pista y que el bloque
// empiece justo en un múltiplo de 15 minutos. Si la pista existe, está abierta o el
// slot está libre lo decide Pistas.
func ValidateSlot(s Slot) error {
	if strings.TrimSpace(s.RunwayID) == "" {
		return fmt.Errorf("%w: falta la pista", ErrInvalidFlight)
	}
	start := s.BlockStart.UTC()
	if !start.Truncate(BlockDuration).Equal(start) {
		return fmt.Errorf("%w: el bloque tiene que empezar en un múltiplo de 15 minutos", ErrInvalidFlight)
	}
	return nil
}

// NewFlightData son los datos que manda la aerolínea al publicar (F-02).
type NewFlightData struct {
	Number      string
	Type        FlightType
	Scope       Scope
	Origin      string
	Destination string
	ScheduledAt time.Time
}

// Flight es un vuelo de una aerolínea en nuestro aeropuerto.
type Flight struct {
	ID          string
	Number      string
	Airline     string
	Type        FlightType
	Scope       Scope
	Origin      string
	Destination string
	ScheduledAt time.Time
	Status      Status
	Slot        *Slot
	DivertedTo  string
	// Version sube con cada cambio guardado. Los eventos la llevan como versión
	// del dato, para que el que los recibe descarte uno viejo (4.8).
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewFlight valida los datos y crea el vuelo en "pendiente de slot", sin slot (RN-06).
// Rechaza origen igual al destino (RN-40) y un horario anterior a now (RN-41).
func NewFlight(id, airline string, d NewFlightData, now time.Time) (*Flight, error) {
	if strings.TrimSpace(airline) == "" {
		return nil, fmt.Errorf("%w: falta la aerolínea", ErrInvalidFlight)
	}
	if !flightNumberPattern.MatchString(d.Number) {
		return nil, fmt.Errorf("%w: %q no es un número de vuelo válido", ErrInvalidFlight, d.Number)
	}
	if d.Type != Arrival && d.Type != Departure {
		return nil, fmt.Errorf("%w: el tipo tiene que ser arrival o departure", ErrInvalidFlight)
	}
	if d.Scope != Domestic && d.Scope != International {
		return nil, fmt.Errorf("%w: el ámbito tiene que ser domestic o international", ErrInvalidFlight)
	}
	origin := strings.TrimSpace(d.Origin)
	destination := strings.TrimSpace(d.Destination)
	if origin == "" || destination == "" {
		return nil, fmt.Errorf("%w: faltan el origen o el destino", ErrInvalidFlight)
	}
	if d.Type == Departure && !isHomeCity(origin) {
		return nil, fmt.Errorf("%w: un despegue tiene que salir de %s", ErrInvalidFlight, HomeCity)
	}
	if d.Type == Arrival && !isHomeCity(destination) {
		return nil, fmt.Errorf("%w: un arribo tiene que llegar a %s", ErrInvalidFlight, HomeCity)
	}
	// Nuestra ciudad se guarda siempre igual, para que la búsqueda la encuentre (RN-34).
	if isHomeCity(origin) {
		origin = HomeCity
	}
	if isHomeCity(destination) {
		destination = HomeCity
	}
	if strings.EqualFold(withoutAccents.Replace(origin), withoutAccents.Replace(destination)) {
		return nil, fmt.Errorf("%w: el origen y el destino no pueden ser iguales", ErrInvalidFlight)
	}
	if d.ScheduledAt.IsZero() {
		return nil, fmt.Errorf("%w: falta el horario", ErrInvalidFlight)
	}
	if d.ScheduledAt.Before(now) {
		return nil, fmt.Errorf("%w: el horario ya pasó", ErrInvalidFlight)
	}

	return &Flight{
		ID:          id,
		Number:      d.Number,
		Airline:     airline,
		Type:        d.Type,
		Scope:       d.Scope,
		Origin:      origin,
		Destination: destination,
		ScheduledAt: d.ScheduledAt.UTC(),
		Status:      PendingSlot,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// withoutAccents saca los tildes, para aceptar "Cordoba" igual que "Córdoba".
var withoutAccents = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u",
	"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U",
)

func isHomeCity(city string) bool {
	return strings.EqualFold(withoutAccents.Replace(city), withoutAccents.Replace(HomeCity))
}

// CheckOwner rechaza la operación si el vuelo es de otra aerolínea (RN-30).
func (f *Flight) CheckOwner(airline string) error {
	if f.Airline != airline {
		return ErrNotOwner
	}
	return nil
}

// CanRequestSlot dice si se le puede pedir un slot a Pistas para este vuelo: solo en
// "pendiente de slot" o "demorado". Un vuelo programado no cambia de slot (RN-28).
func (f *Flight) CanRequestSlot() error {
	if !CanTransition(f.Status, Scheduled, f.Type) {
		return fmt.Errorf("%w: un vuelo %s no puede pedir slot", ErrInvalidTransition, f.Status)
	}
	return nil
}

// AssignSlot aplica el evento "slot asignado": el vuelo queda programado (RN-08, RN-19).
func (f *Flight) AssignSlot(s Slot, now time.Time) error {
	if err := f.moveTo(Scheduled, now); err != nil {
		return err
	}
	f.Slot = &Slot{RunwayID: s.RunwayID, BlockStart: s.BlockStart.UTC()}
	return nil
}

// LoseSlotToArrival aplica el evento "slot cedido a un arribo": el despegue pierde su
// slot y queda demorado (RN-11, RN-12).
func (f *Flight) LoseSlotToArrival(now time.Time) error {
	if f.Type != Departure {
		return fmt.Errorf("%w: solo un despegue cede su slot", ErrInvalidTransition)
	}
	if err := f.moveTo(Delayed, now); err != nil {
		return err
	}
	f.Slot = nil
	return nil
}

// ReleaseByClosure aplica el evento "slot liberado por cierre". El despegue queda
// demorado y sin slot (RN-17); el arribo queda desviado al alternativo que eligió
// Pistas (RN-18, RN-25).
func (f *Flight) ReleaseByClosure(alternate string, now time.Time) error {
	if f.Type == Departure {
		if err := f.moveTo(Delayed, now); err != nil {
			return err
		}
		f.Slot = nil
		return nil
	}
	if !icaoPattern.MatchString(alternate) {
		return fmt.Errorf("%w: %q no es un código ICAO", ErrInvalidFlight, alternate)
	}
	if err := f.moveTo(Diverted, now); err != nil {
		return err
	}
	f.DivertedTo = alternate
	return nil
}

// Complete da por finalizado un vuelo programado cuando termina su bloque (RN-32).
func (f *Flight) Complete(now time.Time) error {
	if f.Slot != nil && now.Before(f.Slot.End()) {
		return fmt.Errorf("%w: el bloque todavía no terminó", ErrInvalidTransition)
	}
	return f.moveTo(Completed, now)
}

// Cancel cancela el vuelo. Solo puede la aerolínea dueña (RN-31). El slot lo libera
// Pistas al recibir "vuelo cancelado" (RN-23).
func (f *Flight) Cancel(airline string, now time.Time) error {
	if err := f.CheckOwner(airline); err != nil {
		return err
	}
	if err := f.moveTo(Cancelled, now); err != nil {
		return err
	}
	f.Slot = nil
	return nil
}

// EffectiveTime es el horario que muestra el tablero: el del bloque si el vuelo tiene
// slot, y el publicado si no (RN-38, RN-39).
func (f *Flight) EffectiveTime() time.Time {
	if f.Slot != nil {
		return f.Slot.BlockStart
	}
	return f.ScheduledAt
}

func (f *Flight) moveTo(to Status, now time.Time) error {
	if !CanTransition(f.Status, to, f.Type) {
		return fmt.Errorf("%w: de %s a %s para un %s", ErrInvalidTransition, f.Status, to, f.Type)
	}
	f.Status = to
	f.UpdatedAt = now
	return nil
}
