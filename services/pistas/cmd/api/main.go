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

	"github.com/gin-gonic/gin"

	httpadapter "github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/adapters/http"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/adapters/mysql"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/adapters/rabbitmq"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/observability"
)

const servicio = "pistas"

func main() {
	// El logger se arma antes que nada, para que hasta un error de arranque salga en JSON (11.5).
	log := observability.NewLogger(servicio)
	slog.SetDefault(log)
	if err := correr(log); err != nil {
		log.Error("el servicio se detuvo con error", "error", err)
		os.Exit(1)
	}
}

func correr(log *slog.Logger) error {
	port := variable("PORT", "8080")
	dsn := variable("MYSQL_DSN", "")
	rabbitURL := variable("RABBITMQ_URL", "")
	if dsn == "" || rabbitURL == "" {
		return errors.New("faltan MYSQL_DSN o RABBITMQ_URL")
	}

	ctx, cancelar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelar()

	// Trazas, métricas y logs a Grafana (11.1). Sin OTEL_EXPORTER_OTLP_ENDPOINT, solo logs.
	apagarTelemetria, err := observability.Setup(ctx, servicio)
	if err != nil {
		return err
	}
	defer func() {
		envio, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelar()
		if err := apagarTelemetria(envio); err != nil {
			log.Error("no se pudo enviar la telemetría pendiente", "error", err)
		}
	}()

	repo, err := conectarConEspera(ctx, dsn, log)
	if err != nil {
		return err
	}

	publicador := rabbitmq.NuevoPublicador(rabbitURL)
	defer publicador.Cerrar()
	go aplicacion.NuevoRelay(repo, publicador, log).Correr(ctx)

	// Sin GIN_MODE, Gin escribe avisos en texto que se mezclarían con los logs JSON.
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}
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
		return err
	}
	return nil
}

// conectarConEspera reintenta mientras MySQL arranca. En Docker Compose, Pistas ya espera a que la
// base esté sana; esto cubre el arranque fuera de Docker y un reinicio de la base.
func conectarConEspera(ctx context.Context, dsn string, log *slog.Logger) (*mysql.Repositorio, error) {
	var ultimo error
	for intento := 1; intento <= 30; intento++ {
		repo, err := mysql.Conectar(dsn)
		if err == nil {
			if err = repo.Verificar(ctx); err == nil {
				return repo, nil
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
