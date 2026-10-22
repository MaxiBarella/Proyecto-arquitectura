package aplicacion

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type claveCorrelacion struct{}

// ConCorrelacion guarda en el contexto el identificador de correlación del pedido (11.2).
func ConCorrelacion(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, claveCorrelacion{}, id)
}

// CorrelacionDe devuelve el identificador de correlación del contexto, o uno nuevo si no hay.
func CorrelacionDe(ctx context.Context) string {
	if id, ok := ctx.Value(claveCorrelacion{}).(string); ok && id != "" {
		return id
	}
	return NuevoIDDeTraza()
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
