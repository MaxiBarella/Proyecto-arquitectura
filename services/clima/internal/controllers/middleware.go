package controllers

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// Límite de pedidos por clave (6.15, 9.17): un balde de 60 que se rellena de a 1 por segundo.
const (
	rateLimitBurst  = 60
	rateLimitPerSec = 1
)

// consumerKey es la clave del contexto de Gin donde queda el consumidor autenticado.
const consumerKey = "consumer"

// RequireAPIKey exige una clave válida en X-API-Key (6.13). keys va de clave a nombre del consumidor.
func RequireAPIKey(keys map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			writeProblem(c, http.StatusUnauthorized, codeMissingAPIKey, "El pedido no incluye el header X-API-Key.", 0)
			return
		}
		consumer, ok := keys[key]
		if !ok {
			writeProblem(c, http.StatusUnauthorized, codeInvalidAPIKey, "La clave enviada no corresponde a ningún consumidor.", 0)
			return
		}
		c.Set(consumerKey, consumer)
		c.Next()
	}
}

// RateLimit aplica el límite por consumidor. Se cuenta en memoria, lo que alcanza
// porque en la nube Clima corre en una sola instancia (9.6).
func RateLimit() gin.HandlerFunc {
	var mu sync.Mutex
	limiters := map[string]*rate.Limiter{}

	return func(c *gin.Context) {
		consumer := c.GetString(consumerKey)
		mu.Lock()
		limiter, ok := limiters[consumer]
		if !ok {
			limiter = rate.NewLimiter(rateLimitPerSec, rateLimitBurst)
			limiters[consumer] = limiter
		}
		mu.Unlock()

		now := time.Now()
		reservation := limiter.ReserveN(now, 1)
		if delay := reservation.DelayFrom(now); delay > 0 {
			reservation.CancelAt(now)
			retryAfter := int(math.Ceil(delay.Seconds()))
			writeProblem(c, http.StatusTooManyRequests, codeRateLimitExceeded,
				"La clave superó los 60 pedidos por minuto.", retryAfter)
			return
		}
		remaining := int(math.Floor(limiter.TokensAt(now)))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(remaining, 0)))
		c.Next()
	}
}

// RequireOperator deja pasar solo pedidos del operador, según el encabezado que
// pone el gateway (15.6).
func RequireOperator() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-User-Role") != "operator" {
			writeProblem(c, http.StatusForbidden, codeForbidden, "Solo el operador puede forzar una condición.", 0)
			return
		}
		c.Next()
	}
}
