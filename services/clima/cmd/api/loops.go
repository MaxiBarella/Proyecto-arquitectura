package main

import (
	"context"
	"log/slog"
	"time"
)

// Cada cuánto se consulta Open-Meteo (9.2) y cada cuánto se revisa si el último
// dato se volvió viejo, para avisar "sin datos" a tiempo.
const (
	pollInterval    = 15 * time.Minute
	refreshInterval = time.Minute
)

// runEvery ejecuta task enseguida y después cada interval, hasta que se cancele
// ctx. Si una ejecución falla, lo registra y sigue: la próxima vuelve a intentar.
func runEvery(ctx context.Context, interval time.Duration, name string, task func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := task(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "falló una tarea periódica", "task", name, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
