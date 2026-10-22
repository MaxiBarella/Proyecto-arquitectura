package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/observability"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/repositories"
)

// ErrNoData indica que no hay un dato de clima reciente para responder (9.2).
var ErrNoData = errors.New("no hay un dato de clima reciente")

// ErrInvalidForced indica que la condición forzada no tiene sentido.
var ErrInvalidForced = errors.New("condición forzada inválida")

// maxConflictRetries es cuántas veces se reintenta guardar el estado si otro
// proceso lo cambió en el medio.
const maxConflictRetries = 3

// WeatherSource trae la observación actual de la fuente externa.
type WeatherSource interface {
	Current(ctx context.Context, airport models.Airport) (models.Observation, error)
}

// ObservationStore guarda y lee observaciones.
type ObservationStore interface {
	Save(ctx context.Context, obs models.Observation) error
	Latest(ctx context.Context, airport string) (models.Observation, error)
}

// AptitudeStore guarda el documento de aptitud con su outbox.
type AptitudeStore interface {
	Get(ctx context.Context, airport string) (models.AptitudeState, error)
	SetForced(ctx context.Context, airport string, forced *models.ForcedCondition) error
	SavePublished(ctx context.Context, airport string, expectedVersion int64, published models.PublishedState, events []models.PendingEvent) error
}

// WeatherService obtiene el clima, calcula la aptitud y decide qué eventos publicar.
type WeatherService struct {
	airport      models.Airport
	source       WeatherSource
	observations ObservationStore
	aptitude     AptitudeStore
	// eventsEnabled es false en la copia de la nube, que no usa RabbitMQ (9.3):
	// el estado se sigue calculando, pero no se encolan eventos.
	eventsEnabled bool
	now           func() time.Time
}

func NewWeatherService(airport models.Airport, source WeatherSource, observations ObservationStore, aptitude AptitudeStore, eventsEnabled bool) *WeatherService {
	return &WeatherService{
		airport:       airport,
		source:        source,
		observations:  observations,
		aptitude:      aptitude,
		eventsEnabled: eventsEnabled,
		now:           time.Now,
	}
}

// Airport devuelve el aeropuerto que atiende el servicio.
func (s *WeatherService) Airport() models.Airport { return s.airport }

// Poll consulta la fuente, guarda la observación y actualiza la aptitud.
func (s *WeatherService) Poll(ctx context.Context) (err error) {
	// Cada consulta es una traza: si cambia la aptitud, el evento y lo que haga
	// Pistas con él quedan dentro de ella (11.6).
	ctx, span := otel.Tracer("clima").Start(ctx, "consulta a Open-Meteo")
	defer func() { observability.EndSpan(span, err) }()

	obs, err := s.source.Current(ctx, s.airport)
	if err != nil {
		return err
	}
	obs.SavedAt = s.now()
	if err := s.observations.Save(ctx, obs); err != nil {
		return err
	}
	slog.InfoContext(ctx, "observación guardada", "airport", obs.Airport, "observed_at", obs.ObservedAt)
	return s.Refresh(ctx)
}

// Current devuelve la última observación real y su aptitud, para la API pública.
// Nunca usa la condición forzada (6.18). Si no hay dato o es viejo, devuelve ErrNoData.
func (s *WeatherService) Current(ctx context.Context) (models.Observation, models.Operability, error) {
	obs, err := s.observations.Latest(ctx, s.airport.ICAO)
	if errors.Is(err, repositories.ErrNotFound) {
		return obs, models.Operability{}, ErrNoData
	}
	if err != nil {
		return obs, models.Operability{}, err
	}
	if IsStale(obs.ObservedAt, s.now()) {
		return obs, models.Operability{}, ErrNoData
	}
	return obs, EvaluateAptitude(obs.Conditions), nil
}

// ForcedInput son los valores que fuerza el operador. Los que no manda se toman
// de la última observación real, así "forzar tormenta" deja el resto como está.
type ForcedInput struct {
	TemperatureC     *float64 `json:"temperature_c"`
	WindSpeedKmh     *float64 `json:"wind_speed_kmh"`
	WindGustKmh      *float64 `json:"wind_gust_kmh"`
	WindDirectionDeg *int     `json:"wind_direction_deg"`
	VisibilityM      *float64 `json:"visibility_m"`
	PrecipitationMm  *float64 `json:"precipitation_mm"`
	Thunderstorm     *bool    `json:"thunderstorm"`
}

// SetForced pone una condición forzada y devuelve la aptitud que resulta (9.12).
func (s *WeatherService) SetForced(ctx context.Context, in ForcedInput) (models.Conditions, models.Operability, error) {
	if err := in.validate(); err != nil {
		return models.Conditions{}, models.Operability{}, err
	}
	base := models.Conditions{VisibilityM: 10000}
	if obs, err := s.observations.Latest(ctx, s.airport.ICAO); err == nil {
		base = obs.Conditions
	} else if !errors.Is(err, repositories.ErrNotFound) {
		return base, models.Operability{}, err
	}
	c := applyForced(base, in)
	forced := &models.ForcedCondition{Conditions: c, SetAt: s.now()}
	if err := s.aptitude.SetForced(ctx, s.airport.ICAO, forced); err != nil {
		return c, models.Operability{}, err
	}
	slog.InfoContext(ctx, "condición forzada", "airport", s.airport.ICAO, "conditions", c)
	if err := s.Refresh(ctx); err != nil {
		return c, models.Operability{}, err
	}
	return c, EvaluateAptitude(c), nil
}

// ClearForced quita la condición forzada y vuelve al dato real.
func (s *WeatherService) ClearForced(ctx context.Context) error {
	if err := s.aptitude.SetForced(ctx, s.airport.ICAO, nil); err != nil {
		return err
	}
	slog.InfoContext(ctx, "condición forzada quitada", "airport", s.airport.ICAO)
	return s.Refresh(ctx)
}

// Refresh recalcula el estado que ve Pistas y, si cambió, encola el evento en el
// outbox. Se llama después de cada consulta, al forzar o quitar una condición,
// y periódicamente para detectar cuándo el dato se vuelve viejo.
func (s *WeatherService) Refresh(ctx context.Context) error {
	for attempt := 0; attempt < maxConflictRetries; attempt++ {
		err := s.refreshOnce(ctx)
		if !errors.Is(err, repositories.ErrConflict) {
			return err
		}
	}
	return fmt.Errorf("no se pudo actualizar la aptitud: %w", repositories.ErrConflict)
}

func (s *WeatherService) refreshOnce(ctx context.Context) error {
	state, err := s.aptitude.Get(ctx, s.airport.ICAO)
	if err != nil {
		return err
	}
	var latest *models.Observation
	obs, err := s.observations.Latest(ctx, s.airport.ICAO)
	switch {
	case err == nil:
		latest = &obs
	case !errors.Is(err, repositories.ErrNotFound):
		return err
	}

	next, ok := effectiveState(state.Forced, latest, s.now())
	if !ok {
		// Nunca hubo dato ni condición forzada: no hay nada que comunicar.
		return nil
	}
	eventType := transition(state.Published, next)
	if eventType == "" && state.Published != nil && samePublished(*state.Published, next) {
		return nil
	}

	var events []models.PendingEvent
	if eventType != "" && s.eventsEnabled {
		events = append(events, s.newEvent(ctx, eventType, state.DataVersion+1, next))
	}
	if err := s.aptitude.SavePublished(ctx, s.airport.ICAO, state.DataVersion, next, events); err != nil {
		return err
	}
	if eventType != "" {
		slog.InfoContext(ctx, "cambió la aptitud que ve Pistas", "airport", s.airport.ICAO,
			"event", eventType, "operable", next.Operable, "no_data", next.NoData, "forced", next.Forced)
	}
	return nil
}

// effectiveState calcula lo que tiene que saber Pistas: la condición forzada si
// existe, si no el dato real, o "sin datos" si el dato real es viejo. El segundo
// valor es false si todavía no hay nada que informar.
func effectiveState(forced *models.ForcedCondition, latest *models.Observation, now time.Time) (models.PublishedState, bool) {
	switch {
	case forced != nil:
		apt := EvaluateAptitude(forced.Conditions)
		return models.PublishedState{Operable: apt.Operable, Reasons: apt.Reasons, ObservedAt: forced.SetAt, Forced: true}, true
	case latest == nil:
		return models.PublishedState{}, false
	case IsStale(latest.ObservedAt, now):
		return models.PublishedState{NoData: true, ObservedAt: latest.ObservedAt}, true
	default:
		apt := EvaluateAptitude(latest.Conditions)
		return models.PublishedState{Operable: apt.Operable, Reasons: apt.Reasons, ObservedAt: latest.ObservedAt}, true
	}
}

// transition decide qué evento corresponde al pasar de prev a next (9.14):
// "sin datos" al quedarse sin dato; "aptitud cambió" la primera vez, al volver
// los datos, o cuando cambia si se puede operar. Si no corresponde ninguno, "".
func transition(prev *models.PublishedState, next models.PublishedState) string {
	switch {
	case next.NoData:
		if prev == nil || !prev.NoData {
			return models.EventNoData
		}
		return ""
	case prev == nil, prev.NoData, prev.Operable != next.Operable:
		return models.EventAptitudeChanged
	default:
		return ""
	}
}

// samePublished dice si dos estados son iguales en todo lo que se guarda, para
// no escribir en la base cuando no cambió nada.
func samePublished(a, b models.PublishedState) bool {
	if a.NoData != b.NoData || a.Operable != b.Operable || a.Forced != b.Forced || !a.ObservedAt.Equal(b.ObservedAt) {
		return false
	}
	if len(a.Reasons) != len(b.Reasons) {
		return false
	}
	for i := range a.Reasons {
		if a.Reasons[i] != b.Reasons[i] {
			return false
		}
	}
	return true
}

func (s *WeatherService) newEvent(ctx context.Context, eventType string, dataVersion int64, st models.PublishedState) models.PendingEvent {
	observedAt := st.ObservedAt
	data := models.EventData{Airport: s.airport.ICAO, ObservedAt: &observedAt}
	if eventType == models.EventAptitudeChanged {
		operable, forced := st.Operable, st.Forced
		data.Operable, data.Reasons, data.Forced = &operable, st.Reasons, &forced
	}
	return models.PendingEvent{
		ID:            newID(),
		Type:          eventType,
		OccurredAt:    s.now(),
		CorrelationID: correlationID(ctx),
		TraceParent:   observability.TraceParent(ctx),
		DataVersion:   dataVersion,
		Data:          data,
	}
}

// correlationID es el identificador de la traza en curso (11.2). Si no hay
// traza, es un identificador nuevo.
func correlationID(ctx context.Context) string {
	if id := observability.TraceID(ctx); id != "" {
		return id
	}
	return newID()
}

// newID genera un identificador aleatorio con forma de UUID versión 4.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func applyForced(c models.Conditions, in ForcedInput) models.Conditions {
	if in.TemperatureC != nil {
		c.TemperatureC = *in.TemperatureC
	}
	if in.WindSpeedKmh != nil {
		c.WindSpeedKmh = *in.WindSpeedKmh
	}
	if in.WindGustKmh != nil {
		c.WindGustKmh = *in.WindGustKmh
	}
	if in.WindDirectionDeg != nil {
		c.WindDirectionDeg = *in.WindDirectionDeg
	}
	if in.VisibilityM != nil {
		c.VisibilityM = *in.VisibilityM
	}
	if in.PrecipitationMm != nil {
		c.PrecipitationMm = *in.PrecipitationMm
	}
	if in.Thunderstorm != nil {
		c.Thunderstorm = *in.Thunderstorm
	}
	return c
}

// validate rechaza un pedido vacío, que congelaría el dato real como forzado sin que
// nadie lo pida, y valores imposibles.
func (in ForcedInput) validate() error {
	if in == (ForcedInput{}) {
		return fmt.Errorf("%w: hay que mandar al menos un valor", ErrInvalidForced)
	}
	nonNegative := map[string]*float64{
		"wind_speed_kmh":   in.WindSpeedKmh,
		"wind_gust_kmh":    in.WindGustKmh,
		"visibility_m":     in.VisibilityM,
		"precipitation_mm": in.PrecipitationMm,
	}
	for _, name := range []string{"wind_speed_kmh", "wind_gust_kmh", "visibility_m", "precipitation_mm"} {
		if v := nonNegative[name]; v != nil && *v < 0 {
			return fmt.Errorf("%w: %s no puede ser negativo", ErrInvalidForced, name)
		}
	}
	if in.WindDirectionDeg != nil && (*in.WindDirectionDeg < 0 || *in.WindDirectionDeg > 360) {
		return fmt.Errorf("%w: wind_direction_deg tiene que estar entre 0 y 360", ErrInvalidForced)
	}
	return nil
}
