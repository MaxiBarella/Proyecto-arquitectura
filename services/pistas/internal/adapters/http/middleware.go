package httpadapter

import (
	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/observability"
)

// traza le pasa al caso de uso la traza que abrió observability.Middleware, que viene de Vuelos en el
// encabezado traceparent. Los eventos la guardan en el outbox: el identificador va en el
// correlation_id (11.2) y el traceparent sigue en el mensaje (11.6). Si no hay traza (por ejemplo,
// sin colector y sin traceparent en el pedido), se genera un identificador.
func traza() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := observability.TraceID(ctx)
		if id == "" {
			id = aplicacion.NuevoIDDeTraza()
		}
		c.Request = c.Request.WithContext(aplicacion.ConTraza(ctx, id, observability.TraceParent(ctx)))
		c.Next()
	}
}
