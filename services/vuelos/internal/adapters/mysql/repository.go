package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// PageSize es la cantidad de vuelos por página (14.4).
const PageSize = 20

// Open abre la conexión a MySQL y verifica que responda. El DSN sale de la
// variable MYSQL_DSN y tiene que incluir parseTime=true&loc=UTC.
func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{
		// Los errores de la base llegan como gorm.ErrDuplicatedKey y similares.
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
		NowFunc:        func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("conectar con MySQL: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("MySQL no responde: %w", err)
	}
	return db, nil
}

// Repository guarda los vuelos y su outbox en MySQL.
type Repository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, now: func() time.Time { return time.Now().UTC() }}
}

// Create guarda un vuelo nuevo y sus eventos en una sola transacción.
func (r *Repository) Create(ctx context.Context, f *domain.Flight, events ...domain.Event) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toRow(f)).Error; err != nil {
			return fmt.Errorf("guardar el vuelo: %w", err)
		}
		return insertEvents(tx, f.Version, events)
	})
}

// Get devuelve un vuelo, o domain.ErrNotFound.
func (r *Repository) Get(ctx context.Context, id string) (*domain.Flight, error) {
	var row flightRow
	err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("leer el vuelo: %w", err)
	}
	return row.toDomain(), nil
}

// ListByAirline devuelve una página de los vuelos de una aerolínea (RN-30),
// ordenados por el horario que vale (RN-39), y cuántos hay en total. Si status
// no está vacío, filtra por ese estado.
func (r *Repository) ListByAirline(ctx context.Context, airline string, status domain.Status, page int) ([]domain.Flight, int64, error) {
	if page < 1 {
		page = 1
	}
	q := r.db.WithContext(ctx).Model(&flightRow{}).Where("airline = ?", airline)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("contar los vuelos: %w", err)
	}
	var rows []flightRow
	err := q.Order("COALESCE(block_start, scheduled_at), id").
		Limit(PageSize).Offset((page - 1) * PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("listar los vuelos: %w", err)
	}
	flights := make([]domain.Flight, len(rows))
	for i := range rows {
		flights[i] = *rows[i].toDomain()
	}
	return flights, total, nil
}

// Change es un cambio sobre un vuelo (domain.Change).
type Change = domain.Change

// Update aplica un cambio a un vuelo dentro de una transacción. Lee el vuelo con
// SELECT ... FOR UPDATE: si otro pedido lo está cambiando, espera a que termine y
// trabaja sobre el estado nuevo. Guarda el vuelo con la versión siguiente y los
// eventos en la misma transacción.
func (r *Repository) Update(ctx context.Context, id string, change Change) (*domain.Flight, error) {
	var updated *domain.Flight
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		f, err := r.applyLocked(tx, id, change)
		updated = f
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ApplyMessage es Update para un consumidor de eventos (4.9). Registra el
// identificador del mensaje en la misma transacción: si ya estaba, no aplica nada
// y devuelve applied en false.
func (r *Repository) ApplyMessage(ctx context.Context, messageID, flightID string, change Change) (applied bool, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		processed := processedRow{MessageID: messageID, ProcessedAt: r.now()}
		if err := tx.Create(&processed).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errAlreadyProcessed
			}
			return fmt.Errorf("registrar el mensaje: %w", err)
		}
		_, err := r.applyLocked(tx, flightID, change)
		return err
	})
	if errors.Is(err, errAlreadyProcessed) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

var errAlreadyProcessed = errors.New("mensaje ya procesado")

func (r *Repository) applyLocked(tx *gorm.DB, id string, change Change) (*domain.Flight, error) {
	var row flightRow
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("leer el vuelo: %w", err)
	}
	f := row.toDomain()
	events, err := change(f)
	if err != nil {
		return nil, err
	}
	f.Version = row.Version + 1
	if err := tx.Save(toRow(f)).Error; err != nil {
		return nil, fmt.Errorf("guardar el vuelo: %w", err)
	}
	if err := insertEvents(tx, f.Version, events); err != nil {
		return nil, err
	}
	return f, nil
}

func insertEvents(tx *gorm.DB, version int64, events []domain.Event) error {
	for _, e := range events {
		data, err := json.Marshal(e.Data)
		if err != nil {
			return fmt.Errorf("armar los datos del evento %s: %w", e.Type, err)
		}
		row := outboxRow{
			ID:            e.ID,
			Type:          e.Type,
			OccurredAt:    e.OccurredAt.UTC(),
			CorrelationID: e.CorrelationID,
			DataVersion:   version,
			// Como texto: MySQL rechaza un JSON que llega como bytes.
			Data: string(data),
		}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("guardar el evento %s en el outbox: %w", e.Type, err)
		}
	}
	return nil
}
