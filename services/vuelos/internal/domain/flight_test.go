package domain

import (
	"errors"
	"testing"
	"time"
)

var (
	now     = time.Date(2026, 10, 19, 18, 0, 0, 0, time.UTC)
	block   = time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)
	slot05  = Slot{RunwayID: "05", BlockStart: block}
	allStat = []Status{PendingSlot, Scheduled, Delayed, Completed, Diverted, Cancelled}
)

func departureData() NewFlightData {
	return NewFlightData{
		Number:      "AR1550",
		Type:        Departure,
		Scope:       Domestic,
		Origin:      "Córdoba",
		Destination: "Buenos Aires",
		ScheduledAt: block,
	}
}

func arrivalData() NewFlightData {
	return NewFlightData{
		Number:      "LA4021",
		Type:        Arrival,
		Scope:       International,
		Origin:      "Santiago",
		Destination: "Córdoba",
		ScheduledAt: block,
	}
}

func newFlight(t *testing.T, d NewFlightData, status Status) *Flight {
	t.Helper()
	f, err := NewFlight("id-1", "AR", d, now)
	if err != nil {
		t.Fatalf("NewFlight: %v", err)
	}
	f.Status = status
	if status == Scheduled {
		s := slot05
		f.Slot = &s
	}
	return f
}

// La tabla completa de la sección 6 del SPEC: cada par de estados, para cada tipo.
func TestCanTransition(t *testing.T) {
	valid := map[FlightType]map[[2]Status]bool{
		Departure: {
			{PendingSlot, Scheduled}: true,
			{PendingSlot, Cancelled}: true,
			{Scheduled, Delayed}:     true,
			{Scheduled, Completed}:   true,
			{Scheduled, Cancelled}:   true,
			{Delayed, Scheduled}:     true,
			{Delayed, Cancelled}:     true,
		},
		Arrival: {
			{PendingSlot, Scheduled}: true,
			{PendingSlot, Cancelled}: true,
			{Scheduled, Diverted}:    true,
			{Scheduled, Completed}:   true,
			{Scheduled, Cancelled}:   true,
			{Delayed, Cancelled}:     true,
		},
	}
	for typ, pairs := range valid {
		for _, from := range allStat {
			for _, to := range allStat {
				want := pairs[[2]Status{from, to}]
				if got := CanTransition(from, to, typ); got != want {
					t.Errorf("%s de %s a %s: got %v, want %v", typ, from, to, got, want)
				}
			}
		}
	}
}

func TestNewFlight(t *testing.T) {
	f, err := NewFlight("id-1", "AR", departureData(), now)
	if err != nil {
		t.Fatalf("NewFlight: %v", err)
	}
	if f.Status != PendingSlot || f.Slot != nil {
		t.Errorf("un vuelo nuevo tiene que quedar pendiente y sin slot (RN-06), got %s %v", f.Status, f.Slot)
	}

	if _, err := NewFlight("id-2", "LA", arrivalData(), now); err != nil {
		t.Errorf("arribo válido: %v", err)
	}
}

func TestNewFlightRejectsInvalidData(t *testing.T) {
	cases := map[string]func(*NewFlightData){
		"número de vuelo inválido":      func(d *NewFlightData) { d.Number = "ar-1550" },
		"tipo inválido":                 func(d *NewFlightData) { d.Type = "transit" },
		"ámbito inválido":               func(d *NewFlightData) { d.Scope = "local" },
		"sin destino":                   func(d *NewFlightData) { d.Destination = " " },
		"sin horario":                   func(d *NewFlightData) { d.ScheduledAt = time.Time{} },
		"despegue que no sale de acá":   func(d *NewFlightData) { d.Origin = "Rosario" },
		"arribo que no llega acá":       func(d *NewFlightData) { d.Type = Arrival },
		"despegue con origen en blanco": func(d *NewFlightData) { d.Origin = "" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			d := departureData()
			change(&d)
			if _, err := NewFlight("id-1", "AR", d, now); !errors.Is(err, ErrInvalidFlight) {
				t.Errorf("got %v, want ErrInvalidFlight", err)
			}
		})
	}

	if _, err := NewFlight("id-1", "", departureData(), now); !errors.Is(err, ErrInvalidFlight) {
		t.Errorf("sin aerolínea: got %v, want ErrInvalidFlight", err)
	}
}

func TestNewFlightAcceptsHomeCityWrittenAnyWay(t *testing.T) {
	for _, city := range []string{"CÓRDOBA", "Cordoba", "cordoba", " córdoba "} {
		d := departureData()
		d.Origin = city
		f, err := NewFlight("id-1", "AR", d, now)
		if err != nil {
			t.Errorf("%q: got %v", city, err)
			continue
		}
		if f.Origin != HomeCity {
			t.Errorf("%q: se tiene que guardar como %q, got %q", city, HomeCity, f.Origin)
		}
	}
}

func TestValidateSlot(t *testing.T) {
	if err := ValidateSlot(slot05); err != nil {
		t.Errorf("slot válido: %v", err)
	}
	if err := ValidateSlot(Slot{RunwayID: "05", BlockStart: block.Add(7 * time.Minute)}); !errors.Is(err, ErrInvalidFlight) {
		t.Errorf("bloque fuera de un múltiplo de 15 minutos: got %v", err)
	}
	if err := ValidateSlot(Slot{BlockStart: block}); !errors.Is(err, ErrInvalidFlight) {
		t.Errorf("sin pista: got %v", err)
	}
	// 11:30 en Córdoba (UTC-3) es 14:30 UTC: tiene que valer igual.
	cordoba := time.FixedZone("ART", -3*60*60)
	if err := ValidateSlot(Slot{RunwayID: "05", BlockStart: block.In(cordoba)}); err != nil {
		t.Errorf("bloque en otra zona horaria: %v", err)
	}
}

func TestCanRequestSlot(t *testing.T) {
	cases := []struct {
		status Status
		ok     bool
	}{
		{PendingSlot, true},
		{Delayed, true},
		{Scheduled, false}, // RN-28
		{Completed, false},
		{Diverted, false},
		{Cancelled, false},
	}
	for _, c := range cases {
		f := newFlight(t, departureData(), c.status)
		err := f.CanRequestSlot()
		if c.ok && err != nil {
			t.Errorf("%s: got %v, want nil", c.status, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s: got %v, want ErrInvalidTransition", c.status, err)
		}
	}
}

func TestAssignSlot(t *testing.T) {
	later := now.Add(time.Minute)
	f := newFlight(t, departureData(), PendingSlot)
	if err := f.AssignSlot(slot05, later); err != nil {
		t.Fatalf("AssignSlot: %v", err)
	}
	if f.Status != Scheduled || f.Slot == nil || *f.Slot != slot05 || !f.UpdatedAt.Equal(later) {
		t.Errorf("el vuelo tiene que quedar programado con el slot (CA-02), got %s %v", f.Status, f.Slot)
	}

	// Un despegue demorado vuelve a programado con un slot nuevo (RN-19, CA-14).
	d := newFlight(t, departureData(), Delayed)
	if err := d.AssignSlot(slot05, later); err != nil || d.Status != Scheduled {
		t.Errorf("demorado: got %s, %v", d.Status, err)
	}

	// Un vuelo cancelado no recibe slot (CA-15).
	c := newFlight(t, departureData(), Cancelled)
	if err := c.AssignSlot(slot05, later); !errors.Is(err, ErrInvalidTransition) || c.Slot != nil {
		t.Errorf("cancelado: got %v, slot %v", err, c.Slot)
	}
}

func TestLoseSlotToArrival(t *testing.T) {
	f := newFlight(t, departureData(), Scheduled)
	if err := f.LoseSlotToArrival(now); err != nil {
		t.Fatalf("LoseSlotToArrival: %v", err)
	}
	if f.Status != Delayed || f.Slot != nil {
		t.Errorf("el despegue tiene que quedar demorado y sin slot (CA-07), got %s %v", f.Status, f.Slot)
	}

	// Un arribo nunca cede su slot (RN-13).
	a := newFlight(t, arrivalData(), Scheduled)
	if err := a.LoseSlotToArrival(now); !errors.Is(err, ErrInvalidTransition) || a.Status != Scheduled {
		t.Errorf("arribo: got %v, %s", err, a.Status)
	}
}

func TestReleaseByClosure(t *testing.T) {
	d := newFlight(t, departureData(), Scheduled)
	if err := d.ReleaseByClosure("", now); err != nil {
		t.Fatalf("despegue: %v", err)
	}
	if d.Status != Delayed || d.Slot != nil {
		t.Errorf("el despegue tiene que quedar demorado y sin slot (CA-12), got %s %v", d.Status, d.Slot)
	}

	a := newFlight(t, arrivalData(), Scheduled)
	if err := a.ReleaseByClosure("SAOU", now); err != nil {
		t.Fatalf("arribo: %v", err)
	}
	if a.Status != Diverted || a.DivertedTo != "SAOU" {
		t.Errorf("el arribo tiene que quedar desviado a SAOU (CA-13, CA-21), got %s %q", a.Status, a.DivertedTo)
	}

	bad := newFlight(t, arrivalData(), Scheduled)
	if err := bad.ReleaseByClosure("", now); !errors.Is(err, ErrInvalidFlight) || bad.Status != Scheduled {
		t.Errorf("arribo sin alternativo: got %v, %s", err, bad.Status)
	}

	pending := newFlight(t, departureData(), PendingSlot)
	if err := pending.ReleaseByClosure("", now); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("vuelo sin slot: got %v", err)
	}
}

func TestComplete(t *testing.T) {
	f := newFlight(t, departureData(), Scheduled)
	if err := f.Complete(block.Add(10 * time.Minute)); !errors.Is(err, ErrInvalidTransition) || f.Status != Scheduled {
		t.Errorf("antes de que termine el bloque: got %v, %s", err, f.Status)
	}
	if err := f.Complete(block.Add(BlockDuration)); err != nil || f.Status != Completed {
		t.Errorf("al terminar el bloque (CA-27): got %v, %s", err, f.Status)
	}

	// Sin slot no se finaliza.
	for _, s := range []Status{PendingSlot, Delayed} {
		p := newFlight(t, departureData(), s)
		if err := p.Complete(block.Add(time.Hour)); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s: got %v", s, err)
		}
	}
}

func TestCancel(t *testing.T) {
	for _, s := range []Status{PendingSlot, Scheduled, Delayed} {
		f := newFlight(t, departureData(), s)
		if err := f.Cancel("AR", now); err != nil {
			t.Errorf("%s: %v", s, err)
		}
		if f.Status != Cancelled || f.Slot != nil {
			t.Errorf("%s: tiene que quedar cancelado y sin slot (CA-17), got %s %v", s, f.Status, f.Slot)
		}
	}

	// Otra aerolínea no lo cancela (RN-31, CA-26).
	other := newFlight(t, departureData(), Scheduled)
	if err := other.Cancel("FO", now); !errors.Is(err, ErrNotOwner) || other.Status != Scheduled {
		t.Errorf("otra aerolínea: got %v, %s", err, other.Status)
	}

	// Los estados finales no se cancelan, tampoco un arribo desviado (CA-15).
	for _, s := range []Status{Completed, Diverted, Cancelled} {
		f := newFlight(t, arrivalData(), s)
		if err := f.Cancel("AR", now); !errors.Is(err, ErrInvalidTransition) || f.Status != s {
			t.Errorf("%s: got %v, %s", s, err, f.Status)
		}
	}
}

func TestEffectiveTime(t *testing.T) {
	f := newFlight(t, departureData(), PendingSlot)
	if !f.EffectiveTime().Equal(block) {
		t.Errorf("sin slot vale el horario publicado, got %v", f.EffectiveTime())
	}

	// Publicado para las 14:30 y con el bloque de las 18:00 (CA-33).
	evening := Slot{RunwayID: "23", BlockStart: time.Date(2026, 10, 23, 18, 0, 0, 0, time.UTC)}
	if err := f.AssignSlot(evening, now); err != nil {
		t.Fatalf("AssignSlot: %v", err)
	}
	if !f.EffectiveTime().Equal(evening.BlockStart) {
		t.Errorf("con slot vale el horario del bloque, got %v", f.EffectiveTime())
	}
}
