package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	driver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/ports"
)

// Códigos de error de MySQL que significan "chocó con otra transacción".
const (
	errEntradaDuplicada = 1062
	errInterbloqueo     = 1213
	errEsperaDeBloqueo  = 1205
)

// Repositorio guarda pistas, slots y el outbox en MySQL con GORM. Implementa ports.Repositorio y
// ports.Outbox.
type Repositorio struct {
	db *gorm.DB
}

var (
	_ ports.Repositorio = (*Repositorio)(nil)
	_ ports.Outbox      = (*Repositorio)(nil)
)

// Conectar abre la base. El DSN tiene que incluir parseTime=true; los horarios se guardan en UTC.
func Conectar(dsn string) (*Repositorio, error) {
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, err
	}
	return &Repositorio{db: db}, nil
}

// Verificar comprueba que la base responde. Las tablas no las crea el servicio: las crea
// db/001_schema.sql cuando MySQL arranca por primera vez (5.6).
func (r *Repositorio) Verificar(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// EnTransaccion implementa ports.Repositorio.
func (r *Repositorio) EnTransaccion(ctx context.Context, fn func(context.Context, ports.Transaccion) error) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, &transaccion{db: tx})
	})
	return traducir(err)
}

// Pistas implementa ports.Lectura.
func (r *Repositorio) Pistas(ctx context.Context) ([]domain.Pista, error) {
	return pistas(r.db.WithContext(ctx))
}

// Alternativos implementa ports.Lectura.
func (r *Repositorio) Alternativos(ctx context.Context) ([]domain.Alternativo, error) {
	return alternativos(r.db.WithContext(ctx))
}

// ClimaActual implementa ports.Lectura.
func (r *Repositorio) ClimaActual(ctx context.Context) (domain.EstadoClima, error) {
	return climaActual(r.db.WithContext(ctx))
}

// SlotsOcupados implementa ports.Lectura.
func (r *Repositorio) SlotsOcupados(ctx context.Context, desde, hasta time.Time) ([]domain.Slot, error) {
	return slotsOcupados(r.db.WithContext(ctx), desde, hasta)
}

// Pendientes implementa ports.Outbox: los eventos sin publicar, en el orden en que se guardaron.
func (r *Repositorio) Pendientes(ctx context.Context, limite int) ([]ports.EventoGuardado, error) {
	var filas []outboxModelo
	err := r.db.WithContext(ctx).Where("publicado_en IS NULL").Order("secuencia").Limit(limite).Find(&filas).Error
	if err != nil {
		return nil, err
	}
	res := make([]ports.EventoGuardado, len(filas))
	for i, f := range filas {
		res[i] = ports.EventoGuardado{
			Evento: domain.Evento{
				ID:            f.ID,
				Tipo:          f.Tipo,
				Fecha:         f.Fecha.UTC(),
				CorrelacionID: f.CorrelacionID,
				TraceParent:   f.TraceParent,
			},
			Secuencia: f.Secuencia,
			DatosJSON: f.Datos,
		}
	}
	return res, nil
}

// MarcarPublicado implementa ports.Outbox.
func (r *Repositorio) MarcarPublicado(ctx context.Context, secuencia int64) error {
	return r.db.WithContext(ctx).Model(&outboxModelo{}).Where("secuencia = ?", secuencia).
		Update("publicado_en", time.Now().UTC()).Error
}

// transaccion implementa ports.Transaccion sobre una transacción de GORM.
type transaccion struct {
	db *gorm.DB
}

func (t *transaccion) Pistas(ctx context.Context) ([]domain.Pista, error) {
	return pistas(t.db.WithContext(ctx))
}

func (t *transaccion) SlotsOcupados(ctx context.Context, desde, hasta time.Time) ([]domain.Slot, error) {
	return slotsOcupados(t.db.WithContext(ctx), desde, hasta)
}

// Pista usa SELECT ... FOR SHARE: los pedidos de slot no se frenan entre sí, pero un cierre de la
// pista, que la bloquea con FOR UPDATE, espera a que terminen. Sin el bloqueo, un pedido podría leer
// la pista abierta y ocupar un slot justo mientras se cierra (ADR-004).
func (t *transaccion) Pista(ctx context.Context, id string) (domain.Pista, error) {
	var p pistaModelo
	err := t.db.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Pista{}, domain.ErrPistaInexistente
	}
	if err != nil {
		return domain.Pista{}, err
	}
	return p.aDominio(), nil
}

// PistasConBloqueo usa SELECT ... FOR UPDATE sobre todas las pistas: un pedido de slot simultáneo
// espera a que termine el cierre o la reapertura.
func (t *transaccion) PistasConBloqueo(ctx context.Context) ([]domain.Pista, error) {
	return pistas(t.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}))
}

func (t *transaccion) GuardarPista(ctx context.Context, p domain.Pista) error {
	m := pistaDesdeDominio(p)
	return t.db.WithContext(ctx).Model(&pistaModelo{}).Where("id = ?", p.ID).
		Updates(map[string]any{"abierta": m.Abierta, "cerrada_por": m.CerradaPor}).Error
}

// SlotsDePista lee con bloqueo, como el resto de la transacción del cierre.
func (t *transaccion) SlotsDePista(ctx context.Context, pistaID string) ([]domain.Slot, error) {
	var filas []slotModelo
	err := t.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("pista_id = ?", pistaID).Order("inicio_bloque").Find(&filas).Error
	if err != nil {
		return nil, err
	}
	res := make([]domain.Slot, len(filas))
	for i, f := range filas {
		res[i] = f.aDominio()
	}
	return res, nil
}

func (t *transaccion) Alternativos(ctx context.Context) ([]domain.Alternativo, error) {
	return alternativos(t.db.WithContext(ctx))
}

func (t *transaccion) ClimaActual(ctx context.Context) (domain.EstadoClima, error) {
	return climaActual(t.db.WithContext(ctx))
}

// CrearPista inserta la pista. Un nombre repetido no es un choque entre transacciones que se
// arregle reintentando: es domain.ErrPistaYaExiste.
func (t *transaccion) CrearPista(ctx context.Context, p domain.Pista) error {
	m := pistaDesdeDominio(p)
	err := t.db.WithContext(ctx).Create(&m).Error
	var e *driver.MySQLError
	if errors.As(err, &e) && e.Number == errEntradaDuplicada {
		return domain.ErrPistaYaExiste
	}
	return traducir(err)
}

// GuardarAlternativo lee el alternativo con bloqueo, así se sabe si existe aunque el valor no
// cambie (MySQL no cuenta como afectada una fila que queda igual).
func (t *transaccion) GuardarAlternativo(ctx context.Context, icao string, disponible bool) (domain.Alternativo, error) {
	var f alternativoModelo
	err := t.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("icao = ?", icao).First(&f).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Alternativo{}, domain.ErrAlternativoInexistente
	}
	if err != nil {
		return domain.Alternativo{}, err
	}
	if f.Disponible != disponible {
		if err := t.db.WithContext(ctx).Model(&alternativoModelo{}).Where("icao = ?", icao).
			Update("disponible", disponible).Error; err != nil {
			return domain.Alternativo{}, err
		}
	}
	return domain.Alternativo{ICAO: f.ICAO, Nombre: f.Nombre, Disponible: disponible}, nil
}

func (t *transaccion) Clima(ctx context.Context) (domain.EstadoClima, error) {
	var c climaModelo
	err := t.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("aeropuerto = ?", domain.NuestroAeropuerto).First(&c).Error
	if err != nil {
		return domain.EstadoClima{}, err
	}
	return domain.EstadoClima{Operable: c.Operable, ConDatos: c.ConDatos}, nil
}

func (t *transaccion) GuardarClima(ctx context.Context, c domain.EstadoClima) error {
	return t.db.WithContext(ctx).Model(&climaModelo{}).Where("aeropuerto = ?", domain.NuestroAeropuerto).
		Updates(map[string]any{"operable": c.Operable, "con_datos": c.ConDatos, "actualizado_en": time.Now().UTC()}).Error
}

// SlotDelVuelo también usa FOR UPDATE: una lectura con bloqueo ve lo último guardado. Una lectura
// común vería la foto de la base del comienzo de la transacción (REPEATABLE READ) y no se enteraría
// de un pedido simultáneo del mismo vuelo que ya terminó.
func (t *transaccion) SlotDelVuelo(ctx context.Context, vueloID string) (*domain.Slot, error) {
	return primerSlot(t.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("vuelo_id = ?", vueloID))
}

// OcupanteConBloqueo usa SELECT ... FOR UPDATE. Si el slot tiene vuelo, bloquea esa fila: otro
// pedido por el mismo slot espera a que esta transacción termine. Si está libre, MySQL bloquea el
// hueco del índice; si dos pedidos llegan a la vez, uno de los dos falla al insertar y se reintenta.
func (t *transaccion) OcupanteConBloqueo(ctx context.Context, pistaID string, inicio time.Time) (*domain.Slot, error) {
	return primerSlot(t.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("pista_id = ? AND inicio_bloque = ?", pistaID, inicio.UTC()))
}

func (t *transaccion) Ocupar(ctx context.Context, s *domain.Slot) error {
	fila := slotModelo{
		PistaID:       s.PistaID,
		InicioBloque:  s.InicioBloque.UTC(),
		VueloID:       s.VueloID,
		TipoOperacion: string(s.TipoOperacion),
		CreadoEn:      time.Now().UTC(),
	}
	if err := t.db.WithContext(ctx).Create(&fila).Error; err != nil {
		return traducir(err)
	}
	s.ID = fila.ID
	return nil
}

func (t *transaccion) Liberar(ctx context.Context, slotID int64) error {
	return t.db.WithContext(ctx).Delete(&slotModelo{}, slotID).Error
}

func (t *transaccion) GuardarEvento(ctx context.Context, e domain.Evento) error {
	datos, err := json.Marshal(e.Datos)
	if err != nil {
		return fmt.Errorf("no se pudieron serializar los datos del evento %s: %w", e.Tipo, err)
	}
	return t.db.WithContext(ctx).Create(&outboxModelo{
		ID:            e.ID,
		Tipo:          e.Tipo,
		Fecha:         e.Fecha.UTC(),
		CorrelacionID: e.CorrelacionID,
		TraceParent:   e.TraceParent,
		Datos:         datos,
	}).Error
}

// MarcarProcesado inserta el id del mensaje; si ya estaba, no inserta nada y devuelve false.
func (t *transaccion) MarcarProcesado(ctx context.Context, mensajeID string) (bool, error) {
	res := t.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&mensajeProcesadoModelo{ID: mensajeID, ProcesadoEn: time.Now().UTC()})
	if res.Error != nil {
		return false, traducir(res.Error)
	}
	return res.RowsAffected == 1, nil
}

func pistas(db *gorm.DB) ([]domain.Pista, error) {
	var filas []pistaModelo
	if err := db.Order("id").Find(&filas).Error; err != nil {
		return nil, err
	}
	res := make([]domain.Pista, len(filas))
	for i, f := range filas {
		res[i] = f.aDominio()
	}
	return res, nil
}

func slotsOcupados(db *gorm.DB, desde, hasta time.Time) ([]domain.Slot, error) {
	var filas []slotModelo
	err := db.Where("inicio_bloque >= ? AND inicio_bloque < ?", desde.UTC(), hasta.UTC()).Find(&filas).Error
	if err != nil {
		return nil, err
	}
	res := make([]domain.Slot, len(filas))
	for i, f := range filas {
		res[i] = f.aDominio()
	}
	return res, nil
}

func primerSlot(db *gorm.DB) (*domain.Slot, error) {
	var f slotModelo
	err := db.Limit(1).Find(&f).Error
	if err != nil {
		return nil, err
	}
	if f.ID == 0 {
		return nil, nil
	}
	s := f.aDominio()
	return &s, nil
}

// traducir convierte los choques entre transacciones en ports.ErrConflicto.
func traducir(err error) error {
	var e *driver.MySQLError
	if errors.As(err, &e) {
		switch e.Number {
		case errEntradaDuplicada, errInterbloqueo, errEsperaDeBloqueo:
			return fmt.Errorf("%w: %v", ports.ErrConflicto, err)
		}
	}
	return err
}

func alternativos(db *gorm.DB) ([]domain.Alternativo, error) {
	var filas []alternativoModelo
	if err := db.Order("orden").Find(&filas).Error; err != nil {
		return nil, err
	}
	res := make([]domain.Alternativo, len(filas))
	for i, f := range filas {
		res[i] = domain.Alternativo{ICAO: f.ICAO, Nombre: f.Nombre, Disponible: f.Disponible}
	}
	return res, nil
}

func climaActual(db *gorm.DB) (domain.EstadoClima, error) {
	var c climaModelo
	if err := db.Where("aeropuerto = ?", domain.NuestroAeropuerto).First(&c).Error; err != nil {
		return domain.EstadoClima{}, err
	}
	return domain.EstadoClima{Operable: c.Operable, ConDatos: c.ConDatos}, nil
}
