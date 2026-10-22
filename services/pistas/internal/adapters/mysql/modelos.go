package mysql

import (
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

// pistaModelo es la tabla de pistas.
type pistaModelo struct {
	ID      string `gorm:"primaryKey;size:10"`
	Abierta bool   `gorm:"not null;default:true"`
}

func (pistaModelo) TableName() string { return "pistas" }

func (p pistaModelo) aDominio() domain.Pista {
	return domain.Pista{ID: p.ID, Abierta: p.Abierta}
}

// slotModelo es la tabla de slots ocupados. Un bloque libre no tiene fila.
//
// Las dos restricciones únicas son las que garantizan las reglas aunque lleguen pedidos a la vez:
//   - ux_slot_pista_bloque: un slot admite un solo vuelo (RN-04, RN-10).
//   - ux_slot_vuelo: un vuelo tiene a lo sumo un slot, que es lo que hace idempotente el pedido (4.6).
type slotModelo struct {
	ID            int64     `gorm:"primaryKey"`
	PistaID       string    `gorm:"size:10;not null;uniqueIndex:ux_slot_pista_bloque,priority:1"`
	InicioBloque  time.Time `gorm:"not null;uniqueIndex:ux_slot_pista_bloque,priority:2;index:ix_slot_inicio"`
	VueloID       string    `gorm:"size:64;not null;uniqueIndex:ux_slot_vuelo"`
	TipoOperacion string    `gorm:"size:10;not null"`
	CreadoEn      time.Time `gorm:"not null"`
}

func (slotModelo) TableName() string { return "slots" }

func (s slotModelo) aDominio() domain.Slot {
	return domain.Slot{
		ID:            s.ID,
		PistaID:       s.PistaID,
		InicioBloque:  s.InicioBloque.UTC(),
		VueloID:       s.VueloID,
		TipoOperacion: domain.TipoOperacion(s.TipoOperacion),
	}
}

// outboxModelo es la tabla de outbox (4.4, 4.7). Secuencia da el orden de publicación y es el
// "version_dato" del mensaje (4.8).
type outboxModelo struct {
	Secuencia     int64      `gorm:"primaryKey;autoIncrement"`
	ID            string     `gorm:"size:36;not null;uniqueIndex"`
	Tipo          string     `gorm:"size:50;not null"`
	Fecha         time.Time  `gorm:"not null"`
	CorrelacionID string     `gorm:"size:64;not null"`
	TraceParent   string     `gorm:"column:traceparent;size:55;not null"`
	Datos         []byte     `gorm:"type:json;not null"`
	PublicadoEn   *time.Time `gorm:"index"`
}

func (outboxModelo) TableName() string { return "outbox" }
