package models

import "time"

// Tipos de evento que publica Clima, usados como clave de ruteo (9.14).
const (
	EventAptitudeChanged = "clima.aptitud.cambio"
	EventNoData          = "clima.sin_datos"
)

// AptitudeState es el documento de aptitud de un aeropuerto (ADR-003). Guarda lo
// que ya se le comunicó a Pistas, la condición forzada si la hay, y los eventos
// pendientes de publicar: el outbox va dentro del mismo documento, así el cambio
// de estado y su evento se guardan en una sola escritura (4.7).
type AptitudeState struct {
	Airport string `bson:"_id"`
	// Forced es la condición que puso el operador. Mientras exista, manda sobre el dato real (18, pendiente 17).
	Forced *ForcedCondition `bson:"forced,omitempty"`
	// Published es el último estado comunicado. Es nil hasta el primer evento.
	Published *PublishedState `bson:"published,omitempty"`
	// DataVersion crece con cada evento; los consumidores descartan los que llegan con una versión vieja (4.8).
	DataVersion int64          `bson:"data_version"`
	Outbox      []PendingEvent `bson:"outbox"`
}

// ForcedCondition son los valores de clima que fuerza el operador (9.12).
type ForcedCondition struct {
	Conditions Conditions `bson:"conditions"`
	SetAt      time.Time  `bson:"set_at"`
}

// PublishedState es lo que Pistas sabe hoy de este aeropuerto.
type PublishedState struct {
	NoData     bool      `bson:"no_data"`
	Operable   bool      `bson:"operable"`
	Reasons    []string  `bson:"reasons"`
	ObservedAt time.Time `bson:"observed_at"`
	Forced     bool      `bson:"forced"`
}

// PendingEvent es un evento guardado en el outbox, a la espera de que el relay lo publique.
type PendingEvent struct {
	ID            string    `bson:"id"`
	Type          string    `bson:"type"`
	OccurredAt    time.Time `bson:"occurred_at"`
	CorrelationID string    `bson:"correlation_id"`
	// TraceParent es la traza en curso al guardar el evento. El relay publica
	// dentro de esa traza, así sigue en el consumidor (11.6).
	TraceParent string    `bson:"traceparent,omitempty"`
	DataVersion int64     `bson:"data_version"`
	Data        EventData `bson:"data"`
}

// EventData son los datos de un evento de Clima (9.14). "Sin datos" solo lleva
// el aeropuerto y la fecha del último dato que hubo.
type EventData struct {
	Airport    string     `bson:"airport" json:"airport"`
	Operable   *bool      `bson:"operable,omitempty" json:"operable,omitempty"`
	Reasons    []string   `bson:"reasons,omitempty" json:"reasons,omitempty"`
	ObservedAt *time.Time `bson:"observed_at,omitempty" json:"observed_at,omitempty"`
	Forced     *bool      `bson:"forced,omitempty" json:"forced,omitempty"`
}
