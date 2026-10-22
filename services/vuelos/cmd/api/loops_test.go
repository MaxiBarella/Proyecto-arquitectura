package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFailureLogNoRepiteElMismoErrorEnElMinuto(t *testing.T) {
	f := failureLog{name: "prueba"}
	start := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	err := errors.New("RabbitMQ no responde")

	f.record(context.Background(), err, start)
	for i := 1; i < 60; i++ {
		f.record(context.Background(), err, start.Add(time.Duration(i)*time.Second))
	}
	if f.skipped != 59 {
		t.Errorf("tendría que haber salteado 59 líneas, salteó %d", f.skipped)
	}
	f.record(context.Background(), err, start.Add(time.Minute))
	if f.skipped != 0 || !f.lastLog.Equal(start.Add(time.Minute)) {
		t.Errorf("pasado el minuto tendría que volver a registrarlo: %+v", f)
	}
}

func TestFailureLogRegistraUnErrorDistinto(t *testing.T) {
	f := failureLog{name: "prueba"}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	f.record(context.Background(), errors.New("uno"), now)
	f.record(context.Background(), errors.New("otro"), now.Add(time.Second))
	if f.lastError != "otro" || f.skipped != 0 {
		t.Errorf("un error distinto se tendría que registrar: %+v", f)
	}
}

func TestFailureLogSeReiniciaAlRecuperarse(t *testing.T) {
	f := failureLog{name: "prueba"}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	f.record(context.Background(), errors.New("uno"), now)
	f.record(context.Background(), nil, now.Add(time.Second))
	if f.failing {
		t.Error("después de un éxito no tendría que figurar fallando")
	}
}

func TestRunEveryReintentaAntesSiFalla(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := make(chan time.Time, 10)
	task := func(context.Context) error {
		calls <- time.Now()
		return errors.New("falla")
	}
	go runEvery(ctx, time.Hour, 20*time.Millisecond, "prueba", task)
	first, second := <-calls, <-calls
	if gap := second.Sub(first); gap > time.Second {
		t.Errorf("después de una falla tendría que reintentar enseguida, tardó %v", gap)
	}
}

func TestRunEveryLeDaUnPlazoACadaEjecucion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deadlines := make(chan bool, 1)
	go runEvery(ctx, time.Hour, time.Hour, "prueba", func(taskCtx context.Context) error {
		_, ok := taskCtx.Deadline()
		deadlines <- ok
		return nil
	})
	if !<-deadlines {
		t.Error("cada ejecución tendría que tener un plazo")
	}
}
