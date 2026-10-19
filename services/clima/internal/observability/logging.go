package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

// NewLogger arma el logger del servicio: una línea JSON por evento en la salida
// estándar y, si hay colector, la misma línea enviada a Grafana. Cada línea lleva
// el nombre del servicio y, si ocurre dentro de un pedido, el identificador de la traza.
func NewLogger(service string) *slog.Logger {
	var handler slog.Handler = traceHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}
	if Enabled() {
		handler = slog.NewMultiHandler(handler, otelslog.NewHandler(service))
	}
	return slog.New(handler).With("service", service)
}

// RequestLogger escribe una línea por cada pedido HTTP con su método, ruta,
// estado y duración. Va después de Middleware, para que la línea lleve la traza,
// y antes de Recovery, para que también quede la línea de un pedido que falló.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isHealthCheck(c.Request) {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		logger.InfoContext(c.Request.Context(), "pedido HTTP",
			"method", c.Request.Method,
			"route", route,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// Recovery reemplaza a gin.Recovery: si un handler entra en pánico, responde 500
// y deja el error en el log como JSON, con la traza, en lugar de texto suelto.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		logger.ErrorContext(c.Request.Context(), "pánico al atender un pedido",
			"error", fmt.Sprint(recovered),
			"stack", string(debug.Stack()),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

// traceHandler agrega trace_id y span_id a cada línea escrita dentro de una traza,
// para poder pasar de un log a su traza en Grafana.
type traceHandler struct {
	slog.Handler
}

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		record = record.Clone()
		record.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{Handler: h.Handler.WithGroup(name)}
}
