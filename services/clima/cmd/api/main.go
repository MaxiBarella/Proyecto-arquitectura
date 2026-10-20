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

	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/controllers"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/observability"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/repositories"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/clima/internal/services"
)

const serviceName = "clima"

// shutdownTimeout es el tiempo máximo de cada etapa del apagado: terminar los
// pedidos en curso, y después enviar la telemetría pendiente.
const shutdownTimeout = 5 * time.Second

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

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	port := cfg.port

	mongoClient, db, err := repositories.Connect(ctx, cfg.mongoURI)
	if err != nil {
		return err
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = mongoClient.Disconnect(disconnectCtx)
	}()

	aptitude := repositories.NewAptitudeRepository(db)
	weather := services.NewWeatherService(
		models.Cordoba,
		repositories.NewOpenMeteo(),
		repositories.NewObservationRepository(db),
		aptitude,
		cfg.rabbitEnabled,
	)
	if cfg.rabbitEnabled {
		publisher := repositories.NewRabbitPublisher(cfg.rabbitURL)
		defer publisher.Close()
		relay := services.NewRelay(aptitude, publisher)
		go runEvery(ctx, relayInterval, relayInterval, "publicación de eventos", relay.PublishPending)
	}
	go runEvery(ctx, pollInterval, pollRetryInterval, "consulta a Open-Meteo", weather.Poll)
	go runEvery(ctx, refreshInterval, refreshInterval, "revisión de la aptitud", weather.Refresh)

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

	conditions := controllers.NewConditionsController(weather)
	public := router.Group("", controllers.RequireAPIKey(cfg.apiKeys), controllers.RateLimit())
	public.GET("/airports/:icao/conditions", conditions.Get)
	if cfg.rabbitEnabled {
		// La ruta del forzado solo existe en la copia del Docker Compose (9.13).
		internal := router.Group("/internal", controllers.RequireOperator())
		internal.PUT("/airports/:icao/forced-conditions", conditions.PutForced)
		internal.DELETE("/airports/:icao/forced-conditions", conditions.DeleteForced)
	}

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
