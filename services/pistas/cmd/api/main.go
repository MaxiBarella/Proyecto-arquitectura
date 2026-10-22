package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/adapters/http"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/adapters/mysql"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/adapters/rabbitmq"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("servicio", "pistas")

	port := variable("PORT", "8080")
	dsn := variable("MYSQL_DSN", "")
	rabbitURL := variable("RABBITMQ_URL", "")
	if dsn == "" || rabbitURL == "" {
		log.Error("faltan MYSQL_DSN o RABBITMQ_URL")
		os.Exit(1)
	}

	ctx, cancelar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelar()

	repo, err := conectarConEspera(ctx, dsn, log)
	if err != nil {
		log.Error("no se pudo preparar la base", "error", err)
		os.Exit(1)
	}

	publicador := rabbitmq.NuevoPublicador(rabbitURL)
	defer publicador.Cerrar()
	go aplicacion.NuevoRelay(repo, publicador, log).Correr(ctx)

	servidor := &http.Server{
		Addr:              ":" + port,
		Handler:           httpadapter.NuevoRouter(aplicacion.NuevoPistas(repo), log),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		apagado, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		_ = servidor.Shutdown(apagado)
	}()

	log.Info("pistas escuchando", "puerto", port)
	if err := servidor.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("el servidor se detuvo", "error", err)
		os.Exit(1)
	}
}

// conectarConEspera reintenta mientras MySQL arranca: en Docker Compose el servicio puede levantar
// antes que la base.
func conectarConEspera(ctx context.Context, dsn string, log *slog.Logger) (*mysql.Repositorio, error) {
	var ultimo error
	for intento := 1; intento <= 30; intento++ {
		repo, err := mysql.Conectar(dsn)
		if err == nil {
			if err = repo.Migrar(ctx); err == nil {
				if err = repo.CargarPistasIniciales(ctx); err == nil {
					return repo, nil
				}
			}
		}
		ultimo = err
		log.Warn("la base todavía no responde; se reintenta", "intento", intento, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, ultimo
}

func variable(nombre, porDefecto string) string {
	if v := os.Getenv(nombre); v != "" {
		return v
	}
	return porDefecto
}
