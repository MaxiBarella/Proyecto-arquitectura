// Package notificaciones manda mails con la capacidad que publica Recién hecho, el
// grupo proveedor (27.1). Es el único lugar que conoce su contrato
// (testdata/notificaciones.yaml): si cambia, se toca solo este archivo (ADR-009).
package notificaciones

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

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
)

// DefaultTimeout es el plazo de cada pedido al proveedor. Es el que recomienda su
// guía (27.5).
const DefaultTimeout = 3 * time.Second

// Client llama a POST /notificaciones del proveedor.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient arma el cliente para el proveedor de baseURL, con la clave del grupo.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		// otelhttp suma la llamada al proveedor a la traza en curso (11.1).
		http: &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

// pedido es el cuerpo de POST /notificaciones (NotificacionNueva en su contrato).
type pedido struct {
	Canal        string `json:"canal"`
	Destinatario string `json:"destinatario"`
	Asunto       string `json:"asunto"`
	Cuerpo       string `json:"cuerpo"`
}

// errorProveedor es la forma de sus errores: {"codigo", "mensaje"}.
type errorProveedor struct {
	Codigo  string `json:"codigo"`
	Mensaje string `json:"mensaje"`
}

// Send pide el envío del mail. El proveedor lo acepta con 202 y lo manda en segundo
// plano. Cualquier otra respuesta, o no recibir respuesta, vuelve como error para
// que el consumidor reintente con la misma Idempotency-Key (27.5, 27.6).
func (c *Client) Send(ctx context.Context, m commands.Mail) error {
	body, err := json.Marshal(pedido{Canal: "email", Destinatario: m.To, Asunto: m.Subject, Cuerpo: m.Body})
	if err != nil {
		return fmt.Errorf("armar el pedido al proveedor: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/notificaciones", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Idempotency-Key", m.IdempotencyKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("llamar al proveedor de notificaciones: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	var e errorProveedor
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&e)
	return fmt.Errorf("el proveedor de notificaciones respondió %d %s: %s", resp.StatusCode, e.Codigo, e.Mensaje)
}
