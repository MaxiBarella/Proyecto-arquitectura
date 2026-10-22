package main

import (
	"context"
	"log/slog"
	"time"
)

// relayInterval es cada cuánto el relay revisa el outbox (4.5).
const relayInterval = time.Second

// taskTimeout es el plazo de cada ejecución. Si la base o RabbitMQ se cuelgan, la
// tarea se corta y la próxima vuelve a intentar, en vez de quedar trabada. Es el
// mismo valor que en Clima (9.20); este archivo es una copia del de Clima.
const taskTimeout = 30 * time.Second

// repeatedErrorLogInterval: si una tarea falla seguido con el mismo error, se
// registra como mucho una vez por minuto, para no llenar el log.
const repeatedErrorLogInterval = time.Minute

// runEvery ejecuta task enseguida y después cada interval, hasta que se cancele ctx.
// Si una ejecución falla, la próxima es a los retryInterval.
func runEvery(ctx context.Context, interval, retryInterval time.Duration, name string, task func(context.Context) error) {
	failures := failureLog{name: name}
	for {
		taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)
		err := task(taskCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		failures.record(ctx, err, time.Now())

		wait := interval
		if err != nil {
			wait = retryInterval
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// failureLog decide qué fallas de una tarea registrar: la primera, después el mismo
// error como mucho una vez por minuto, y cuando la tarea se recupera.
type failureLog struct {
	name      string
	failing   bool
	lastError string
	lastLog   time.Time
	skipped   int
}

func (f *failureLog) record(ctx context.Context, err error, now time.Time) {
	if err == nil {
		if f.failing {
			slog.InfoContext(ctx, "una tarea periódica se recuperó", "task", f.name)
		}
		*f = failureLog{name: f.name}
		return
	}
	msg := err.Error()
	if f.failing && msg == f.lastError && now.Sub(f.lastLog) < repeatedErrorLogInterval {
		f.skipped++
		return
	}
	slog.ErrorContext(ctx, "falló una tarea periódica", "task", f.name, "error", msg, "repeated_since_last_log", f.skipped)
	f.failing, f.lastError, f.lastLog, f.skipped = true, msg, now, 0
}
