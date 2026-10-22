package mysql

import (
	"encoding/json"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// flightRow es una fila de la tabla flights (db/001_schema.sql). Las fechas las
// pone el dominio, así que GORM no las completa solo.
type flightRow struct {
	ID           string `gorm:"primaryKey"`
	FlightNumber string
	Airline      string
	Type         string
	Scope        string
	Origin       string
	Destination  string
	ScheduledAt  time.Time
	Status       string
	RunwayID     *string
	BlockStart   *time.Time
	DivertedTo   *string
	Version      int64
	CreatedAt    time.Time `gorm:"autoCreateTime:false"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime:false"`
}

func (flightRow) TableName() string { return "flights" }

type outboxRow struct {
	ID            string `gorm:"primaryKey"`
	Type          string
	OccurredAt    time.Time
	CorrelationID string
	TraceParent   *string `gorm:"column:traceparent"`
	DataVersion   int64
	Data          string
}

func (outboxRow) TableName() string { return "outbox" }

type processedRow struct {
	MessageID   string `gorm:"primaryKey"`
	ProcessedAt time.Time
}

func (processedRow) TableName() string { return "processed_messages" }

func toRow(f *domain.Flight) *flightRow {
	row := &flightRow{
		ID:           f.ID,
		FlightNumber: f.Number,
		Airline:      f.Airline,
		Type:         string(f.Type),
		Scope:        string(f.Scope),
		Origin:       f.Origin,
		Destination:  f.Destination,
		ScheduledAt:  f.ScheduledAt.UTC(),
		Status:       string(f.Status),
		Version:      f.Version,
		CreatedAt:    f.CreatedAt.UTC(),
		UpdatedAt:    f.UpdatedAt.UTC(),
	}
	if f.Slot != nil {
		runway, start := f.Slot.RunwayID, f.Slot.BlockStart.UTC()
		row.RunwayID, row.BlockStart = &runway, &start
	}
	if f.DivertedTo != "" {
		alternate := f.DivertedTo
		row.DivertedTo = &alternate
	}
	return row
}

func (row *flightRow) toDomain() *domain.Flight {
	f := &domain.Flight{
		ID:          row.ID,
		Number:      row.FlightNumber,
		Airline:     row.Airline,
		Type:        domain.FlightType(row.Type),
		Scope:       domain.Scope(row.Scope),
		Origin:      row.Origin,
		Destination: row.Destination,
		ScheduledAt: row.ScheduledAt.UTC(),
		Status:      domain.Status(row.Status),
		Version:     row.Version,
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
	if row.RunwayID != nil && row.BlockStart != nil {
		f.Slot = &domain.Slot{RunwayID: *row.RunwayID, BlockStart: row.BlockStart.UTC()}
	}
	if row.DivertedTo != nil {
		f.DivertedTo = *row.DivertedTo
	}
	return f
}

func (row outboxRow) toEvent() domain.Event {
	e := domain.Event{
		ID:            row.ID,
		Type:          row.Type,
		OccurredAt:    row.OccurredAt.UTC(),
		CorrelationID: row.CorrelationID,
		DataVersion:   row.DataVersion,
		// Los datos ya están en JSON: se publican tal cual.
		Data: json.RawMessage(row.Data),
	}
	if row.TraceParent != nil {
		e.TraceParent = *row.TraceParent
	}
	return e
}
