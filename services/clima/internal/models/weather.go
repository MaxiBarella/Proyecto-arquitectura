package models

import "time"

// Conditions son los valores de clima de una observación, en unidades métricas (6.16).
type Conditions struct {
	TemperatureC     float64 `json:"temperature_c" bson:"temperature_c"`
	WindSpeedKmh     float64 `json:"wind_speed_kmh" bson:"wind_speed_kmh"`
	WindGustKmh      float64 `json:"wind_gust_kmh" bson:"wind_gust_kmh"`
	WindDirectionDeg int     `json:"wind_direction_deg" bson:"wind_direction_deg"`
	VisibilityM      float64 `json:"visibility_m" bson:"visibility_m"`
	PrecipitationMm  float64 `json:"precipitation_mm" bson:"precipitation_mm"`
	Thunderstorm     bool    `json:"thunderstorm" bson:"thunderstorm"`
}

// Motivos por los que un aeropuerto no es apto (6.11).
const (
	ReasonHighWind      = "HIGH_WIND"
	ReasonHighGusts     = "HIGH_GUSTS"
	ReasonLowVisibility = "LOW_VISIBILITY"
	ReasonThunderstorm  = "THUNDERSTORM"
)

// Operability dice si se puede operar y, si no, por qué.
type Operability struct {
	Operable bool     `json:"operable" bson:"operable"`
	Reasons  []string `json:"reasons" bson:"reasons"`
}

// Observation es una lectura de Open-Meteo guardada en MongoDB.
type Observation struct {
	Airport    string     `bson:"airport"`
	ObservedAt time.Time  `bson:"observed_at"`
	Conditions Conditions `bson:"conditions"`
	// SavedAt es cuándo se guardó. El índice TTL la usa para borrar las de más de 7 días (9.16).
	SavedAt time.Time `bson:"saved_at"`
}

// Airport es un aeropuerto atendido por Clima.
type Airport struct {
	ICAO      string
	Name      string
	Latitude  float64
	Longitude float64
}

// Cordoba es el único aeropuerto atendido (2.8, 6.9).
var Cordoba = Airport{
	ICAO:      "SACO",
	Name:      "Aeropuerto Internacional Ingeniero Aeronáutico Ambrosio Taravella",
	Latitude:  -31.3236,
	Longitude: -64.2080,
}
