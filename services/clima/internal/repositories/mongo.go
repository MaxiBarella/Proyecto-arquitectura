package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
)

const databaseName = "clima"

// observationTTL es cuánto se guarda cada observación antes de que MongoDB la borre (9.16).
const observationTTL = 7 * 24 * time.Hour

// ErrNotFound indica que no hay ningún documento que cumpla la búsqueda.
var ErrNotFound = errors.New("no encontrado")

// ErrConflict indica que el documento cambió desde que se leyó.
var ErrConflict = errors.New("el documento cambió mientras se actualizaba")

// Connect abre la conexión a MongoDB, verifica que responda y crea los índices.
func Connect(ctx context.Context, uri string) (*mongo.Client, *mongo.Database, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, fmt.Errorf("conectar con MongoDB: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("MongoDB no responde: %w", err)
	}
	db := client.Database(databaseName)
	if err := createIndexes(ctx, db); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, err
	}
	return client, db, nil
}

func createIndexes(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection("observations").Indexes().CreateMany(ctx, []mongo.IndexModel{
		// Para leer la última observación de un aeropuerto (ADR-003).
		{Keys: bson.D{{Key: "airport", Value: 1}, {Key: "observed_at", Value: -1}}},
		// Borra las observaciones viejas (9.16).
		{
			Keys:    bson.D{{Key: "saved_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(int32(observationTTL.Seconds())),
		},
	})
	if err != nil {
		return fmt.Errorf("crear los índices de observaciones: %w", err)
	}
	return nil
}

// ObservationRepository guarda y lee las observaciones de clima.
type ObservationRepository struct {
	coll *mongo.Collection
}

func NewObservationRepository(db *mongo.Database) *ObservationRepository {
	return &ObservationRepository{coll: db.Collection("observations")}
}

// Save agrega una observación.
func (r *ObservationRepository) Save(ctx context.Context, obs models.Observation) error {
	if _, err := r.coll.InsertOne(ctx, obs); err != nil {
		return fmt.Errorf("guardar la observación: %w", err)
	}
	return nil
}

// Latest devuelve la observación más reciente de un aeropuerto, o ErrNotFound.
func (r *ObservationRepository) Latest(ctx context.Context, airport string) (models.Observation, error) {
	var obs models.Observation
	opts := options.FindOne().SetSort(bson.D{{Key: "observed_at", Value: -1}})
	err := r.coll.FindOne(ctx, bson.D{{Key: "airport", Value: airport}}, opts).Decode(&obs)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return obs, ErrNotFound
	}
	if err != nil {
		return obs, fmt.Errorf("leer la última observación: %w", err)
	}
	return obs, nil
}

// AptitudeRepository guarda el documento de aptitud de cada aeropuerto, con su outbox.
type AptitudeRepository struct {
	coll *mongo.Collection
}

func NewAptitudeRepository(db *mongo.Database) *AptitudeRepository {
	return &AptitudeRepository{coll: db.Collection("aptitude")}
}

// Get devuelve el documento de un aeropuerto. Si todavía no existe, devuelve uno vacío.
func (r *AptitudeRepository) Get(ctx context.Context, airport string) (models.AptitudeState, error) {
	state := models.AptitudeState{Airport: airport}
	err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: airport}}).Decode(&state)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("leer el documento de aptitud: %w", err)
	}
	return state, nil
}

// SetForced pone o quita (con nil) la condición forzada.
func (r *AptitudeRepository) SetForced(ctx context.Context, airport string, forced *models.ForcedCondition) error {
	update := bson.D{{Key: "$unset", Value: bson.D{{Key: "forced", Value: ""}}}}
	if forced != nil {
		update = bson.D{{Key: "$set", Value: bson.D{{Key: "forced", Value: forced}}}}
	}
	_, err := r.coll.UpdateOne(ctx, bson.D{{Key: "_id", Value: airport}}, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("guardar la condición forzada: %w", err)
	}
	return nil
}

// SavePublished guarda el nuevo estado comunicado y sus eventos en una sola
// escritura. Solo la aplica si nadie cambió el documento desde que se leyó con
// la versión expectedVersion; si no, devuelve ErrConflict.
func (r *AptitudeRepository) SavePublished(ctx context.Context, airport string, expectedVersion int64, published models.PublishedState, events []models.PendingEvent) error {
	set := bson.D{
		{Key: "published", Value: published},
		{Key: "data_version", Value: expectedVersion + int64(len(events))},
	}
	update := bson.D{{Key: "$set", Value: set}}
	if len(events) > 0 {
		update = append(update, bson.E{Key: "$push", Value: bson.D{
			{Key: "outbox", Value: bson.D{{Key: "$each", Value: events}}},
		}})
	}
	filter := bson.D{{Key: "_id", Value: airport}, {Key: "data_version", Value: expectedVersion}}
	if expectedVersion == 0 {
		// El documento puede no existir, o existir solo con la condición forzada.
		filter = bson.D{{Key: "_id", Value: airport}, {Key: "data_version", Value: bson.D{{Key: "$in", Value: bson.A{0, nil}}}}}
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		// El upsert quiso crear un documento que ya existe con otra versión.
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("guardar el estado de aptitud: %w", err)
	}
	return nil
}

// PendingEvents devuelve los documentos que tienen eventos sin publicar.
func (r *AptitudeRepository) PendingEvents(ctx context.Context) ([]models.AptitudeState, error) {
	filter := bson.D{{Key: "outbox.0", Value: bson.D{{Key: "$exists", Value: true}}}}
	cursor, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("buscar eventos pendientes: %w", err)
	}
	var states []models.AptitudeState
	if err := cursor.All(ctx, &states); err != nil {
		return nil, fmt.Errorf("leer eventos pendientes: %w", err)
	}
	return states, nil
}

// MarkPublished saca un evento del outbox una vez publicado.
func (r *AptitudeRepository) MarkPublished(ctx context.Context, airport, eventID string) error {
	update := bson.D{{Key: "$pull", Value: bson.D{{Key: "outbox", Value: bson.D{{Key: "id", Value: eventID}}}}}}
	if _, err := r.coll.UpdateOne(ctx, bson.D{{Key: "_id", Value: airport}}, update); err != nil {
		return fmt.Errorf("sacar el evento del outbox: %w", err)
	}
	return nil
}
