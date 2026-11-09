// Package memcached guarda en Memcached las páginas del tablero público (10, ADR-007).
package memcached

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

// TTL es cuánto dura cada página guardada: solo vencimiento, sin borrar con los
// eventos (10.3).
const TTL = 60 * time.Second

// Timeout es cuánto se espera a Memcached. Pasado ese plazo se va directo a Solr
// (10.4, 10.5).
const Timeout = 100 * time.Millisecond

// maxIdleConns son las conexiones que se dejan abiertas para reusar. Con las 2 que trae
// la librería, a cientos de pedidos por segundo abriría una conexión nueva casi en cada
// uno; con 100 se reusan.
const maxIdleConns = 100

// Resultados de la caché, para la métrica de aciertos y fallos (ADR-007).
const (
	resultHit   = "hit"
	resultMiss  = "miss"
	resultError = "error"
)

// BoardCache envuelve el índice del tablero: si la página está en Memcached la
// devuelve de ahí, y si no la busca en el índice y la guarda (ADR-007).
type BoardCache struct {
	next    queries.BoardIndex
	mc      *memcache.Client
	lookups metric.Int64Counter
}

// NewBoardCache arma la caché delante de next, con el Memcached de addr (host:puerto).
func NewBoardCache(next queries.BoardIndex, addr string) *BoardCache {
	mc := memcache.New(addr)
	mc.Timeout = Timeout
	mc.MaxIdleConns = maxIdleConns
	lookups, _ := otel.Meter("vuelos").Int64Counter("vuelos.tablero.cache",
		metric.WithDescription("Consultas del tablero a la caché, por resultado: hit, miss o error (ADR-007)."))
	return &BoardCache{next: next, mc: mc, lookups: lookups}
}

// page es lo que se guarda: una página del tablero y el total.
type page struct {
	Flights []queries.IndexedFlight `json:"flights"`
	Total   int64                   `json:"total"`
}

// Search devuelve la página guardada o, si no está, la busca en el índice y la guarda.
// Si Memcached falla, sigue igual contra el índice: la caché nunca es imprescindible (10.4).
func (c *BoardCache) Search(ctx context.Context, s queries.BoardSearch) ([]queries.IndexedFlight, int64, error) {
	key := Key(s)
	item, err := c.mc.Get(key)
	switch {
	case err == nil:
		var p page
		if jsonErr := json.Unmarshal(item.Value, &p); jsonErr == nil {
			c.count(ctx, resultHit)
			return p.Flights, p.Total, nil
		}
		// Algo roto guardado: se trata como un fallo y se pisa con la página nueva.
		c.count(ctx, resultMiss)
	case errors.Is(err, memcache.ErrCacheMiss):
		c.count(ctx, resultMiss)
	default:
		c.count(ctx, resultError)
		slog.WarnContext(ctx, "Memcached no responde; el tablero va directo a Solr", "error", err)
	}

	flights, total, err := c.next.Search(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	if value, jsonErr := json.Marshal(page{Flights: flights, Total: total}); jsonErr == nil {
		setErr := c.mc.Set(&memcache.Item{Key: key, Value: value, Expiration: int32(TTL / time.Second)})
		if setErr != nil {
			slog.WarnContext(ctx, "no se pudo guardar la página del tablero en Memcached", "error", setErr)
		}
	}
	return flights, total, nil
}

func (c *BoardCache) count(ctx context.Context, result string) {
	c.lookups.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

// Key es la clave de una consulta: "tablero:" más el sha1 de la consulta normalizada
// (ADR-007). El texto a buscar es libre, y Memcached no acepta claves con espacios ni de
// más de 250 caracteres. La consulta ya viene con los valores por defecto completos
// (página 1, orden por horario y el rango de fechas calculado), así la misma búsqueda
// da siempre la misma clave. El texto se pasa a minúsculas porque la búsqueda no
// distingue mayúsculas (14.8).
func Key(s queries.BoardSearch) string {
	to := ""
	if !s.To.IsZero() {
		to = s.To.UTC().Format(time.RFC3339)
	}
	normalized := strings.Join([]string{
		"type=" + s.Type,
		"status=" + s.Status,
		"airline=" + s.Airline,
		"q=" + strings.ToLower(s.Query),
		"sort=" + s.Sort,
		"page=" + strconv.Itoa(s.Page),
		"from=" + s.From.UTC().Format(time.RFC3339),
		"to=" + to,
	}, "&")
	sum := sha1.Sum([]byte(normalized))
	return fmt.Sprintf("tablero:%s", hex.EncodeToString(sum[:]))
}
