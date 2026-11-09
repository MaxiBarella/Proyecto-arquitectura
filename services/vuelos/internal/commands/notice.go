package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// EventFlightClosureHit es el evento que deja Vuelos cuando el cierre de una pista
// le demora o le desvía un vuelo (RN-17, RN-18). Lo lee Vuelos mismo, de la cola
// vuelos.avisos, para avisarle por mail a la aerolínea con el proveedor (27.1, 27.4).
const EventFlightClosureHit = "vuelos.vuelo.afectado_por_cierre"

// ClosureHitData son los datos del aviso: lo que hace falta para escribir el mail.
type ClosureHitData struct {
	FlightID     string    `json:"flight_id"`
	FlightNumber string    `json:"flight_number"`
	Airline      string    `json:"airline"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Origin       string    `json:"origin"`
	Destination  string    `json:"destination"`
	RunwayID     string    `json:"runway_id"`
	BlockStart   time.Time `json:"block_start"`
	DivertedTo   *string   `json:"diverted_to"`
}

// closureHit es el aviso del vuelo f, ya demorado o desviado. La pista y el bloque
// salen del evento de Pistas, porque el despegue demorado ya no tiene slot.
func (s *Service) closureHit(ctx context.Context, f *domain.Flight, released slotEventData) domain.Event {
	d := ClosureHitData{
		FlightID:     f.ID,
		FlightNumber: f.Number,
		Airline:      f.Airline,
		Type:         string(f.Type),
		Status:       string(f.Status),
		Origin:       f.Origin,
		Destination:  f.Destination,
		RunwayID:     released.RunwayID,
		BlockStart:   released.BlockStart.UTC(),
	}
	if f.DivertedTo != "" {
		alt := f.DivertedTo
		d.DivertedTo = &alt
	}
	return s.event(ctx, EventFlightClosureHit, d)
}

// Mail es un mail para una aerolínea. IdempotencyKey hace que un reintento no lo
// mande dos veces (27.6).
type Mail struct {
	IdempotencyKey string
	To             string
	Subject        string
	Body           string
}

// Mailer manda un mail con el proveedor. Un error se reintenta (27.5).
type Mailer interface {
	Send(ctx context.Context, m Mail) error
}

// Notifier le avisa por mail a la aerolínea cuando el cierre de una pista le
// demora o le desvía un vuelo (27.1).
type Notifier struct {
	mailer Mailer
	// emails va del código de la aerolínea a su dirección (27.3).
	emails map[string]string
}

func NewNotifier(mailer Mailer, emails map[string]string) *Notifier {
	return &Notifier{mailer: mailer, emails: emails}
}

// HandleEvent manda el mail del aviso. Devuelve ErrUnprocessable si el mensaje no
// es un aviso válido o la aerolínea no tiene mail cargado: reintentar no lo
// arregla. Cualquier otro error es del proveedor y el consumidor reintenta.
func (n *Notifier) HandleEvent(ctx context.Context, m Message) error {
	if m.Type != EventFlightClosureHit {
		return fmt.Errorf("%w: la cola de avisos no espera %s", ErrUnprocessable, m.Type)
	}
	var d ClosureHitData
	if err := json.Unmarshal(m.Data, &d); err != nil || d.FlightID == "" {
		return fmt.Errorf("%w: datos inválidos del aviso: %v", ErrUnprocessable, err)
	}
	to, ok := n.emails[d.Airline]
	if !ok {
		return fmt.Errorf("%w: la aerolínea %s no tiene mail cargado en AEROLINEAS_EMAILS", ErrUnprocessable, d.Airline)
	}
	subject, body, err := writeMail(d)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnprocessable, err)
	}
	// El id del evento es único por aviso y no cambia entre reintentos (27.6).
	if err := n.mailer.Send(ctx, Mail{IdempotencyKey: m.ID, To: to, Subject: subject, Body: body}); err != nil {
		return err
	}
	slog.InfoContext(ctx, "aviso a la aerolínea aceptado por el proveedor",
		"flight_id", d.FlightID, "airline", d.Airline, "status", d.Status, "event_id", m.ID)
	return nil
}

// writeMail arma el asunto y el texto. Los horarios van en hora de Argentina,
// como el tablero (14.9).
func writeMail(d ClosureHitData) (subject, body string, err error) {
	block := d.BlockStart.In(argentina).Format("02/01 15:04")
	switch domain.Status(d.Status) {
	case domain.Delayed:
		subject = fmt.Sprintf("Vuelo %s demorado por el cierre de la pista %s", d.FlightNumber, d.RunwayID)
		body = fmt.Sprintf("Su despegue %s de %s a %s perdió el slot de la pista %s de las %s porque la pista se cerró.\n\n"+
			"El vuelo quedó demorado. Para que vuelva a quedar programado, pida un slot nuevo desde el sistema del aeropuerto.",
			d.FlightNumber, d.Origin, d.Destination, d.RunwayID, block)
	case domain.Diverted:
		alt := ""
		if d.DivertedTo != nil {
			alt = *d.DivertedTo
		}
		subject = fmt.Sprintf("Vuelo %s desviado a %s por el cierre de la pista %s", d.FlightNumber, alt, d.RunwayID)
		body = fmt.Sprintf("Su arribo %s de %s a %s tenía la pista %s a las %s, pero la pista se cerró.\n\n"+
			"El vuelo fue desviado al aeropuerto alternativo %s.",
			d.FlightNumber, d.Origin, d.Destination, d.RunwayID, block, alt)
	default:
		return "", "", errors.New("el aviso es para un vuelo demorado o desviado, no " + d.Status)
	}
	return strings.TrimSpace(subject), body, nil
}

// argentina es la hora de Argentina (UTC−3), sin horario de verano.
var argentina = time.FixedZone("ART", -3*60*60)
