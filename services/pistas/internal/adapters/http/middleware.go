package httpadapter

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
)

// correlacion toma el identificador de la traza del encabezado traceparent de W3C, que es el que
// propaga OpenTelemetry (11.2). Si el pedido no lo trae, genera uno.
func correlacion() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := idDeTraza(c.GetHeader("traceparent"))
		if id == "" {
			id = aplicacion.NuevoIDDeTraza()
		}
		c.Request = c.Request.WithContext(aplicacion.ConCorrelacion(c.Request.Context(), id))
		c.Next()
	}
}

// idDeTraza extrae el trace id de "00-<trace id>-<span id>-<flags>".
func idDeTraza(traceparent string) string {
	partes := strings.Split(traceparent, "-")
	if len(partes) != 4 || len(partes[1]) != 32 {
		return ""
	}
	return partes[1]
}

// registrarPedido escribe una línea de log por pedido (11.5).
func registrarPedido(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		inicio := time.Now()
		c.Next()
		if c.Request.URL.Path == "/health" {
			return
		}
		log.InfoContext(c.Request.Context(), "pedido atendido",
			"metodo", c.Request.Method,
			"ruta", c.FullPath(),
			"estado", c.Writer.Status(),
			"duracion_ms", time.Since(inicio).Milliseconds(),
			"correlacion_id", aplicacion.CorrelacionDe(c.Request.Context()),
		)
	}
}
