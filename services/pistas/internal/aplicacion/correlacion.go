package aplicacion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type claveTraza struct{}

// traza es lo que un evento necesita saber de la traza del pedido que lo causó.
type traza struct {
	correlacion string
	traceparent string
}

// ConTraza guarda en el contexto la traza del pedido: el identificador de la traza, que va en el
// correlation_id del evento (11.2), y el encabezado traceparent, que se guarda en el outbox para que
// la traza siga en el mensaje (11.6). La pone el adaptador HTTP; así el núcleo no depende de
// OpenTelemetry.
func ConTraza(ctx context.Context, correlacion, traceparent string) context.Context {
	return context.WithValue(ctx, claveTraza{}, traza{correlacion: correlacion, traceparent: traceparent})
}

// CorrelacionDe devuelve el identificador de correlación del contexto, o uno nuevo si no hay.
func CorrelacionDe(ctx context.Context) string {
	if t, ok := ctx.Value(claveTraza{}).(traza); ok && t.correlacion != "" {
		return t.correlacion
	}
	return NuevoIDDeTraza()
}

// TraceParentDe devuelve el traceparent del contexto, o "" si el pedido no tenía traza.
func TraceParentDe(ctx context.Context) string {
	if t, ok := ctx.Value(claveTraza{}).(traza); ok {
		return t.traceparent
	}
	return ""
}

// NuevoIDDeTraza genera un identificador con el formato de un trace id de W3C: 32 caracteres hex.
func NuevoIDDeTraza() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NuevoUUID genera un UUID versión 4, para el id de cada mensaje.
func NuevoUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
