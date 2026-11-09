package solr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

// DefaultTimeout es el plazo de cada pedido a Solr. Provisorio: es el mismo que usa
// Vuelos con Pistas (4.16) y falta que lo confirme el grupo.
const DefaultTimeout = 2 * time.Second

// commitWithin es en cuánto Solr hace visible lo escrito (14.7), en milisegundos.
const commitWithin = 5000

// Client escribe y consulta el índice del tablero en un core de Solr.
type Client struct {
	// coreURL es la dirección del core, por ejemplo http://solr:8983/solr/vuelos.
	coreURL string
	http    *http.Client
	// searches cuenta las consultas del tablero que llegan a Solr, para comparar con
	// y sin caché (ADR-007).
	searches metric.Int64Counter
}

// NewClient arma el cliente para el core de coreURL.
func NewClient(coreURL string, timeout time.Duration) *Client {
	searches, _ := otel.Meter("vuelos").Int64Counter("vuelos.solr.consultas",
		metric.WithDescription("Consultas del tablero que llegan a Solr (ADR-007)."))
	return &Client{
		coreURL: strings.TrimRight(coreURL, "/"),
		// otelhttp suma el pedido a Solr a la traza en curso (11.1).
		http:     &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
		searches: searches,
	}
}

// document es un vuelo como lo guarda Solr. Los nombres de los campos son los del
// esquema del core (ADR-006).
type document struct {
	ID             string     `json:"id"`
	FlightNumber   string     `json:"flight_number"`
	Airline        string     `json:"airline"`
	Type           string     `json:"type"`
	Scope          string     `json:"scope"`
	Origin         string     `json:"origin"`
	Destination    string     `json:"destination"`
	ScheduledAt    time.Time  `json:"scheduled_at"`
	BoardAt        time.Time  `json:"board_at"`
	Status         string     `json:"status"`
	SlotRunwayID   *string    `json:"slot_runway_id,omitempty"`
	SlotBlockStart *time.Time `json:"slot_block_start,omitempty"`
	DivertedTo     *string    `json:"diverted_to,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DataVersion    int64      `json:"data_version"`
}

func toDocument(f queries.IndexedFlight) document {
	d := document{
		ID:           f.ID,
		FlightNumber: f.FlightNumber,
		Airline:      f.Airline,
		Type:         f.Type,
		Scope:        f.Scope,
		Origin:       f.Origin,
		Destination:  f.Destination,
		ScheduledAt:  f.ScheduledAt.UTC(),
		BoardAt:      f.BoardAt.UTC(),
		Status:       f.Status,
		DivertedTo:   f.DivertedTo,
		CreatedAt:    f.CreatedAt.UTC(),
		UpdatedAt:    f.UpdatedAt.UTC(),
		DataVersion:  f.DataVersion,
	}
	if f.Slot != nil {
		runway, start := f.Slot.RunwayID, f.Slot.BlockStart.UTC()
		d.SlotRunwayID, d.SlotBlockStart = &runway, &start
	}
	return d
}

// Upsert guarda el vuelo en el índice, reemplazando el documento entero. Si Solr ya
// tiene una versión igual o más nueva, la descarta sin error (14.6): lo hace la
// cadena de actualización del core, con data_version como versión.
func (c *Client) Upsert(ctx context.Context, f queries.IndexedFlight) error {
	body, err := json.Marshal([]document{toDocument(f)})
	if err != nil {
		return fmt.Errorf("armar el documento de Solr: %w", err)
	}
	url := fmt.Sprintf("%s/update?commitWithin=%d&wt=json", c.coreURL, commitWithin)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("escribir en Solr: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("escribir en Solr: respondió %d: %s", resp.StatusCode, bytes.TrimSpace(detail))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
