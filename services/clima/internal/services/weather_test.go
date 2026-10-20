package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/repositories"
)

// Dobles en memoria de la fuente y de MongoDB.

type fakeSource struct {
	obs models.Observation
	err error
}

func (f *fakeSource) Current(context.Context, models.Airport) (models.Observation, error) {
	return f.obs, f.err
}

type fakeObservations struct{ saved []models.Observation }

func (f *fakeObservations) Save(_ context.Context, obs models.Observation) error {
	f.saved = append(f.saved, obs)
	return nil
}

func (f *fakeObservations) Latest(context.Context, string) (models.Observation, error) {
	if len(f.saved) == 0 {
		return models.Observation{}, repositories.ErrNotFound
	}
	return f.saved[len(f.saved)-1], nil
}

type fakeAptitude struct{ state models.AptitudeState }

func (f *fakeAptitude) Get(context.Context, string) (models.AptitudeState, error) {
	return f.state, nil
}

func (f *fakeAptitude) SetForced(_ context.Context, _ string, forced *models.ForcedCondition) error {
	f.state.Forced = forced
	return nil
}

func (f *fakeAptitude) SavePublished(_ context.Context, _ string, expected int64, published models.PublishedState, events []models.PendingEvent) error {
	if expected != f.state.DataVersion {
		return repositories.ErrConflict
	}
	f.state.Published = &published
	f.state.DataVersion += int64(len(events))
	f.state.Outbox = append(f.state.Outbox, events...)
	return nil
}

// harness arma el servicio con un reloj que el test controla.
type harness struct {
	svc      *WeatherService
	source   *fakeSource
	obs      *fakeObservations
	aptitude *fakeAptitude
	now      time.Time
}

func newHarness(eventsEnabled bool) *harness {
	h := &harness{
		source:   &fakeSource{},
		obs:      &fakeObservations{},
		aptitude: &fakeAptitude{state: models.AptitudeState{Airport: "SACO"}},
		now:      time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
	}
	h.svc = NewWeatherService(models.Cordoba, h.source, h.obs, h.aptitude, eventsEnabled)
	h.svc.now = func() time.Time { return h.now }
	return h
}

// poll simula una consulta a Open-Meteo con estas condiciones, observadas ahora.
func (h *harness) poll(t *testing.T, c models.Conditions) {
	t.Helper()
	h.source.obs = models.Observation{Airport: "SACO", ObservedAt: h.now, Conditions: c}
	if err := h.svc.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}
}

func (h *harness) eventTypes() []string {
	var types []string
	for _, e := range h.aptitude.state.Outbox {
		types = append(types, e.Type)
	}
	return types
}

func storm() models.Conditions {
	c := calm
	c.Thunderstorm = true
	return c
}

func assertEvents(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("eventos = %v, se esperaba %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("eventos = %v, se esperaba %v", got, want)
		}
	}
}

func TestPrimeraObservacionPublicaLaAptitud(t *testing.T) {
	h := newHarness(true)
	h.poll(t, calm)
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged)

	e := h.aptitude.state.Outbox[0]
	if e.Data.Operable == nil || !*e.Data.Operable {
		t.Errorf("el evento tendría que decir operable=true: %+v", e.Data)
	}
	if e.DataVersion != 1 || e.CorrelationID == "" || e.ID == "" {
		t.Errorf("evento incompleto: %+v", e)
	}
}

func TestSoloPublicaCuandoCambiaSiSePuedeOperar(t *testing.T) {
	h := newHarness(true)
	h.poll(t, calm)

	// Sigue apto, con otro viento: no hay evento nuevo.
	h.now = h.now.Add(15 * time.Minute)
	windy := calm
	windy.WindSpeedKmh = 30
	h.poll(t, windy)
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged)

	// Deja de ser apto: evento.
	h.now = h.now.Add(15 * time.Minute)
	h.poll(t, storm())
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventAptitudeChanged)

	// Sigue sin ser apto, por otro motivo: no hay evento nuevo.
	h.now = h.now.Add(15 * time.Minute)
	fog := calm
	fog.VisibilityM = 200
	h.poll(t, fog)
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventAptitudeChanged)

	versions := []int64{h.aptitude.state.Outbox[0].DataVersion, h.aptitude.state.Outbox[1].DataVersion}
	if versions[0] != 1 || versions[1] != 2 {
		t.Errorf("las versiones del dato tendrían que ser 1 y 2: %v", versions)
	}
}

func TestDatoViejoPublicaSinDatosUnaSolaVez(t *testing.T) {
	h := newHarness(true)
	h.poll(t, calm)

	// A los 30 minutos todavía no es viejo.
	h.now = h.now.Add(30 * time.Minute)
	if err := h.svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged)

	// Pasados los 30 minutos, "sin datos".
	h.now = h.now.Add(time.Minute)
	if err := h.svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventNoData)

	// Revisarlo de nuevo no lo repite.
	h.now = h.now.Add(time.Minute)
	if err := h.svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventNoData)

	// Cuando vuelven los datos, se publica la aptitud aunque siga siendo apto.
	h.poll(t, calm)
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventNoData, models.EventAptitudeChanged)
}

func TestLaCondicionForzadaMandaSobreElDatoReal(t *testing.T) {
	h := newHarness(true)
	h.poll(t, calm)

	thunder := true
	_, apt, err := h.svc.SetForced(context.Background(), ForcedInput{Thunderstorm: &thunder})
	if err != nil {
		t.Fatal(err)
	}
	if apt.Operable {
		t.Error("con tormenta forzada no se tendría que poder operar")
	}
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventAptitudeChanged)
	if last := h.aptitude.state.Outbox[1]; last.Data.Forced == nil || !*last.Data.Forced {
		t.Errorf("el evento tendría que decir que es forzado: %+v", last.Data)
	}

	// Un dato real nuevo no la pisa.
	h.now = h.now.Add(15 * time.Minute)
	h.poll(t, calm)
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventAptitudeChanged)

	// Aunque el dato real se vuelva viejo, manda la forzada: no hay "sin datos".
	h.now = h.now.Add(time.Hour)
	if err := h.svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventAptitudeChanged)

	// Al quitarla, vuelve el dato real, que ya es viejo.
	if err := h.svc.ClearForced(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertEvents(t, h.eventTypes(), models.EventAptitudeChanged, models.EventAptitudeChanged, models.EventNoData)
}

func TestLaApiPublicaNoMuestraLaCondicionForzada(t *testing.T) {
	h := newHarness(true)
	h.poll(t, calm)
	thunder := true
	if _, _, err := h.svc.SetForced(context.Background(), ForcedInput{Thunderstorm: &thunder}); err != nil {
		t.Fatal(err)
	}

	obs, apt, err := h.svc.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !apt.Operable || obs.Conditions.Thunderstorm {
		t.Errorf("la API pública tendría que mostrar el dato real: %+v %+v", obs.Conditions, apt)
	}
}

func TestLaApiPublicaRespondeSinDatosSiElDatoEsViejo(t *testing.T) {
	h := newHarness(true)
	if _, _, err := h.svc.Current(context.Background()); !errors.Is(err, ErrNoData) {
		t.Errorf("sin observaciones tendría que ser ErrNoData, fue %v", err)
	}

	h.poll(t, calm)
	h.now = h.now.Add(31 * time.Minute)
	if _, _, err := h.svc.Current(context.Background()); !errors.Is(err, ErrNoData) {
		t.Errorf("con un dato de 31 minutos tendría que ser ErrNoData, fue %v", err)
	}
}

func TestSinRabbitMQNoSeEncolanEventos(t *testing.T) {
	h := newHarness(false)
	h.poll(t, calm)
	h.now = h.now.Add(15 * time.Minute)
	h.poll(t, storm())
	if len(h.aptitude.state.Outbox) != 0 {
		t.Errorf("la copia de la nube no tendría que encolar eventos: %v", h.eventTypes())
	}
	if h.aptitude.state.Published == nil || h.aptitude.state.Published.Operable {
		t.Errorf("el estado se tendría que seguir actualizando: %+v", h.aptitude.state.Published)
	}
}

func TestSiFallaLaFuenteNoSeGuardaNada(t *testing.T) {
	h := newHarness(true)
	h.source.err = errors.New("Open-Meteo no responde")
	if err := h.svc.Poll(context.Background()); err == nil {
		t.Fatal("se esperaba un error")
	}
	if len(h.obs.saved) != 0 || len(h.aptitude.state.Outbox) != 0 {
		t.Error("no se tendría que haber guardado nada")
	}
}

func TestForzadoRechazaValoresImposibles(t *testing.T) {
	neg, dir := -5.0, 400
	tests := map[string]ForcedInput{
		"pedido vacío":         {},
		"viento negativo":      {WindSpeedKmh: &neg},
		"ráfagas negativas":    {WindGustKmh: &neg},
		"visibilidad negativa": {VisibilityM: &neg},
		"lluvia negativa":      {PrecipitationMm: &neg},
		"dirección de 400°":    {WindDirectionDeg: &dir},
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			h := newHarness(true)
			h.poll(t, calm)
			if _, _, err := h.svc.SetForced(context.Background(), in); !errors.Is(err, ErrInvalidForced) {
				t.Errorf("se esperaba ErrInvalidForced, fue %v", err)
			}
			if h.aptitude.state.Forced != nil {
				t.Error("no se tendría que haber guardado la condición forzada")
			}
		})
	}
}

// conflictingAptitude simula que otro proceso cambia el documento entre que el
// servicio lo lee y lo guarda, las primeras conflicts veces.
type conflictingAptitude struct {
	fakeAptitude
	conflicts int
}

func (f *conflictingAptitude) SavePublished(ctx context.Context, airport string, expected int64, published models.PublishedState, events []models.PendingEvent) error {
	if f.conflicts > 0 {
		f.conflicts--
		return repositories.ErrConflict
	}
	return f.fakeAptitude.SavePublished(ctx, airport, expected, published, events)
}

func TestRefreshReintentaAnteUnConflicto(t *testing.T) {
	h := newHarness(true)
	apt := &conflictingAptitude{fakeAptitude: fakeAptitude{state: models.AptitudeState{Airport: "SACO"}}, conflicts: 2}
	h.svc.aptitude = apt
	h.poll(t, calm)
	if len(apt.state.Outbox) != 1 {
		t.Errorf("después de 2 conflictos tendría que haberse guardado el evento: %v", apt.state.Outbox)
	}
}

func TestRefreshSeRindeDespuesDeTresConflictos(t *testing.T) {
	h := newHarness(true)
	apt := &conflictingAptitude{fakeAptitude: fakeAptitude{state: models.AptitudeState{Airport: "SACO"}}, conflicts: 3}
	h.svc.aptitude = apt
	h.source.obs = models.Observation{Airport: "SACO", ObservedAt: h.now, Conditions: calm}
	if err := h.svc.Poll(context.Background()); !errors.Is(err, repositories.ErrConflict) {
		t.Errorf("se esperaba ErrConflict, fue %v", err)
	}
}
