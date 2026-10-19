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

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/observability"
)

const serviceName = "clima"

func main() {
	if err := run(); err != nil {
		slog.Error("el servicio se detuvo con error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Se cancela con Ctrl+C o cuando Docker pide apagar el contenedor.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := observability.Setup(ctx, serviceName)
	if err != nil {
		return err
	}
	logger := observability.NewLogger(serviceName)
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Sin GIN_MODE, Gin escribe avisos en texto que se mezclarían con los logs JSON.
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(
		observability.Middleware(serviceName),
		gin.Recovery(),
		observability.RequestLogger(logger),
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
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	// Al apagar: se terminan los pedidos en curso y se envía la telemetría pendiente.
	logger.Info("apagando el servidor")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return errors.Join(server.Shutdown(shutdownCtx), shutdownTelemetry(shutdownCtx))
}
