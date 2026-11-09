package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/models"
)

// ErrNotFound indica que no hay un usuario con ese nombre.
var ErrNotFound = errors.New("usuario no encontrado")

// Open se conecta a MySQL con GORM (5.4) y verifica que la base responda.
func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{
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

// userRow es una fila de la tabla users (db/001_schema.sql).
type userRow struct {
	ID           string `gorm:"primaryKey"`
	Username     string
	PasswordHash string
	Role         string
	Airline      *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (userRow) TableName() string { return "users" }

// UserRepository lee y guarda los usuarios en MySQL.
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByUsername devuelve el usuario con ese nombre, o ErrNotFound.
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (models.User, error) {
	var row userRow
	err := r.db.WithContext(ctx).Where("username = ?", username).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("buscar el usuario: %w", err)
	}
	return toModel(row), nil
}

// Upsert crea el usuario o, si el nombre ya existe, le actualiza la contraseña,
// el rol y la aerolínea. Así las cuentas de prueba toman la contraseña del .env
// cada vez que arranca el servicio, aunque haya cambiado.
func (r *UserRepository) Upsert(ctx context.Context, u models.User) error {
	row := toRow(u)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoUpdates: clause.AssignmentColumns([]string{"password_hash", "role", "airline", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("guardar el usuario %s: %w", u.Username, err)
	}
	return nil
}

func toRow(u models.User) userRow {
	row := userRow{
		ID:           u.ID,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
	if u.Airline != "" {
		airline := u.Airline
		row.Airline = &airline
	}
	return row
}

func toModel(row userRow) models.User {
	u := models.User{
		ID:           row.ID,
		Username:     row.Username,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	if row.Airline != nil {
		u.Airline = *row.Airline
	}
	return u
}
