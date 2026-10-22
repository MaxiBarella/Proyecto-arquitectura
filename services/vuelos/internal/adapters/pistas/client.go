// Package pistas le pide slots a Pistas por HTTP (4.1), con timeout, reintentos
// (4.5) y circuit breaker (13.1). El contrato está en services/pistas/openapi.yaml.
package pistas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/domain"
)

// Config son los valores de la llamada a Pistas.
type Config struct {
	BaseURL string
	// AttemptTimeout es el plazo de cada intento (4.5, 4.16).
	AttemptTimeout time.Duration
	// MaxRetries es cuántas veces se reintenta ante una falla pasajera (4.5).
	MaxRetries int
	// FirstBackoff es la espera antes del primer reintento. Se duplica en cada
	// uno, con una variación al azar (4.5).
	FirstBackoff time.Duration
	// BreakerFailures es cuántos pedidos seguidos tienen que fallar para que el
	// circuit breaker se abra (13.2).
	BreakerFailures uint32
	// BreakerOpenFor es cuánto tiempo queda abierto antes de dejar pasar un
	// pedido de prueba (13.2).
	BreakerOpenFor time.Duration
}

// DefaultConfig son los valores elegidos en 4.5, 4.16 y 13.2.
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL:         baseURL,
		AttemptTimeout:  2 * time.Second,
		MaxRetries:      2,
		FirstBackoff:    200 * time.Millisecond,
		BreakerFailures: 5,
		BreakerOpenFor:  30 * time.Second,
	}
}

// Client es el cliente de Pistas. El circuit breaker vive en cada instancia de
// Vuelos: cada una decide por su cuenta si deja de llamar a Pistas (13.1).
type Client struct {
	cfg     Config
	http    *http.Client
	breaker *gobreaker.CircuitBreaker[domain.Slot]
}

func NewClient(cfg Config) *Client {
	c := &Client{
		cfg: cfg,
		// otelhttp agrega el encabezado traceparent: la traza sigue en Pistas (11.2).
		http: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
	c.breaker = gobreaker.NewCircuitBreaker[domain.Slot](gobreaker.Settings{
		Name: "pistas",
		// Medio abierto: deja pasar un solo pedido de prueba.
		MaxRequests: 1,
		Timeout:     cfg.BreakerOpenFor,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.BreakerFailures
		},
		// Un rechazo de Pistas (slot ocupado, pista cerrada) es una respuesta
		// correcta: no cuenta como falla.
		IsSuccessful: func(err error) bool {
			return !errors.Is(err, commands.ErrRunwaysUnavailable)
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Warn("cambió el circuit breaker", "breaker", name, "from", from.String(), "to", to.String())
		},
	})
	return c
}

// RequestSlot le pide el slot a Pistas. Si el circuit breaker está abierto, no la
// llama y devuelve commands.ErrRunwaysUnavailable enseguida.
func (c *Client) RequestSlot(ctx context.Context, req commands.SlotRequest) (domain.Slot, error) {
	slot, err := c.breaker.Execute(func() (domain.Slot, error) {
		return c.withRetries(ctx, req)
	})
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return domain.Slot{}, fmt.Errorf("%w: el circuit breaker está abierto", commands.ErrRunwaysUnavailable)
	}
	return slot, err
}

// State devuelve el estado del circuit breaker, para los tests y los logs.
func (c *Client) State() gobreaker.State { return c.breaker.State() }

// withRetries reintenta solo las fallas pasajeras: sin respuesta, error de
// conexión, 502, 503 o 504 (4.5). Un rechazo no se reintenta.
func (c *Client) withRetries(ctx context.Context, req commands.SlotRequest) (domain.Slot, error) {
	wait := c.cfg.FirstBackoff
	for attempt := 0; ; attempt++ {
		slot, err := c.once(ctx, req)
		if !errors.Is(err, errTransient) {
			return slot, err
		}
		if attempt == c.cfg.MaxRetries || ctx.Err() != nil {
			return domain.Slot{}, fmt.Errorf("%w: %v", commands.ErrRunwaysUnavailable, err)
		}
		slog.WarnContext(ctx, "reintento del pedido de slot", "attempt", attempt+1, "error", err)
		// Espera creciente con variación al azar, para que dos instancias no
		// reintenten al mismo tiempo.
		select {
		case <-time.After(wait + rand.N(wait/2+1)):
		case <-ctx.Done():
			return domain.Slot{}, fmt.Errorf("%w: %v", commands.ErrRunwaysUnavailable, ctx.Err())
		}
		wait *= 2
	}
}

var errTransient = errors.New("falla pasajera")

type slotRequestBody struct {
	FlightID   string    `json:"flight_id"`
	FlightType string    `json:"flight_type"`
	RunwayID   string    `json:"runway_id"`
	BlockStart time.Time `json:"block_start"`
}

type slotBody struct {
	RunwayID   string    `json:"runway_id"`
	BlockStart time.Time `json:"block_start"`
}

type problemBody struct {
	Code             string     `json:"code"`
	Detail           string     `json:"detail"`
	NearestFreeSlots []slotBody `json:"nearest_free_slots"`
}

// once hace un intento, con su propio plazo (4.16).
func (c *Client) once(ctx context.Context, req commands.SlotRequest) (domain.Slot, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.AttemptTimeout)
	defer cancel()

	body, err := json.Marshal(slotRequestBody{
		FlightID:   req.FlightID,
		FlightType: string(req.FlightType),
		RunwayID:   req.Slot.RunwayID,
		BlockStart: req.Slot.BlockStart.UTC(),
	})
	if err != nil {
		return domain.Slot{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/internal/slots", bytes.NewReader(body))
	if err != nil {
		return domain.Slot{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		// Sin respuesta: error de conexión o se venció el plazo.
		return domain.Slot{}, fmt.Errorf("%w: %v", errTransient, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var assigned slotBody
		if err := json.NewDecoder(resp.Body).Decode(&assigned); err != nil {
			return domain.Slot{}, fmt.Errorf("%w: respuesta inválida de Pistas: %v", commands.ErrRunwaysUnavailable, err)
		}
		return domain.Slot{RunwayID: assigned.RunwayID, BlockStart: assigned.BlockStart.UTC()}, nil
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return domain.Slot{}, fmt.Errorf("%w: Pistas respondió %d", errTransient, resp.StatusCode)
	case http.StatusNotFound, http.StatusConflict, http.StatusBadRequest:
		return domain.Slot{}, rejection(resp)
	default:
		// Otro error de Pistas (por ejemplo, 500): no se reintenta, pero cuenta
		// como falla para el circuit breaker.
		return domain.Slot{}, fmt.Errorf("%w: Pistas respondió %d", commands.ErrRunwaysUnavailable, resp.StatusCode)
	}
}

// rejection traduce un rechazo de Pistas a los errores de los comandos.
func rejection(resp *http.Response) error {
	var p problemBody
	_ = json.NewDecoder(resp.Body).Decode(&p)
	switch p.Code {
	case "SLOT_TAKEN":
		nearest := make([]domain.Slot, len(p.NearestFreeSlots))
		for i, s := range p.NearestFreeSlots {
			nearest[i] = domain.Slot{RunwayID: s.RunwayID, BlockStart: s.BlockStart.UTC()}
		}
		return &commands.SlotTakenError{Nearest: nearest}
	case "RUNWAY_CLOSED":
		return commands.ErrRunwayClosed
	case "RUNWAY_NOT_FOUND":
		return commands.ErrRunwayNotFound
	case "FLIGHT_ALREADY_HAS_SLOT":
		// Pasa si la aerolínea pide otro slot antes de que llegue el evento del primero.
		return fmt.Errorf("%w: el vuelo ya tiene un slot en Pistas", domain.ErrInvalidTransition)
	default:
		// Un 400 quiere decir que Vuelos mandó algo mal: es un error nuestro.
		return fmt.Errorf("Pistas rechazó el pedido con %d %s: %s", resp.StatusCode, p.Code, p.Detail)
	}
}
