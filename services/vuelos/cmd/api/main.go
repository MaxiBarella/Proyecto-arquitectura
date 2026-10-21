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

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/adapters/mysql"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/observability"
)

const serviceName = "vuelos"

// shutdownTimeout es el tiempo máximo de cada etapa del apagado: terminar los
// pedidos en curso, y después enviar la telemetría pendiente.
const shutdownTimeout = 5 * time.Second

// dbConnectTimeout es cuánto se espera a MySQL al arrancar.
const dbConnectTimeout = 10 * time.Second

func main() {
	// El logger se arma antes que nada, para que hasta un error de arranque salga en JSON.
	logger := observability.NewLogger(serviceName)
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		os.Exit(1)
	}
}

func run(logger *slog.Logger) (err error) {
	// Se cancela con Ctrl+C o cuando Docker pide apagar el contenedor.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := observability.Setup(ctx, serviceName)
	if err != nil {
		logger.Error("no se pudo configurar OpenTelemetry", "error", err)
		return err
	}
	// Al salir, por cualquier camino: si hubo un error se registra, y después se
	// envía la telemetría pendiente con un plazo propio.
	defer func() {
		if err != nil {
			logger.Error("el servicio se detuvo con error", "error", err)
		}
		flushCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if flushErr := shutdownTelemetry(flushCtx); flushErr != nil {
			logger.Error("no se pudo enviar la telemetría pendiente", "error", flushErr)
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return errors.New("falta la variable MYSQL_DSN")
	}

	connectCtx, cancel := context.WithTimeout(ctx, dbConnectTimeout)
	db, err := mysql.Open(connectCtx, dsn)
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	}()
	// Lo usan los comandos y los consumidores de eventos, que vienen en otro PR.
	_ = mysql.NewRepository(db)

	// Sin GIN_MODE, Gin escribe avisos en texto que se mezclarían con los logs JSON.
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(
		observability.Middleware(serviceName),
		observability.RequestLogger(logger),
		observability.Recovery(logger),
	)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": serviceName})
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("servidor escuchando", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err = <-serverErr:
		return err
	case <-ctx.Done():
	}
	// A partir de acá, un segundo Ctrl+C corta el programa en el acto.
	stop()

	logger.Info("apagando el servidor")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
