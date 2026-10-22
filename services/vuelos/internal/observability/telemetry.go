// Package observability configura los logs, las métricas y las trazas del servicio.
//
// Es la misma carpeta en todos los servicios: cada uno es un módulo de Go aparte,
// así que se copia en lugar de importarse (ver README.md).
package observability

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// endpointEnv es la variable estándar de OpenTelemetry con la dirección del colector.
const endpointEnv = "OTEL_EXPORTER_OTLP_ENDPOINT"

// Enabled indica si hay un colector configurado. Sin él (por ejemplo, la copia de
// Clima en la nube) el servicio funciona igual y solo escribe los logs en la salida estándar.
func Enabled() bool {
	return os.Getenv(endpointEnv) != ""
}

// Setup registra los proveedores de trazas, métricas y logs que envían todo al
// colector por OTLP sobre HTTP. Devuelve la función que hay que llamar al apagar
// el servicio para no perder lo que quedó sin enviar.
func Setup(ctx context.Context, service string) (func(context.Context) error, error) {
	// El contexto de la traza viaja entre servicios en el encabezado estándar traceparent.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	if !Enabled() {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewSchemaless(attribute.String("service.name", service)))
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(loggerProvider)

	return func(ctx context.Context) error {
		return errors.Join(
			tracerProvider.Shutdown(ctx),
			meterProvider.Shutdown(ctx),
			loggerProvider.Shutdown(ctx),
		)
	}, nil
}

// Middleware crea una traza por cada pedido HTTP y mide su duración
// (métrica http.server.request.duration). Deja afuera /health (ver isHealthCheck).
func Middleware(service string) gin.HandlerFunc {
	return otelgin.Middleware(service, otelgin.WithFilter(func(r *http.Request) bool {
		return !isHealthCheck(r)
	}))
}

// isHealthCheck indica si el pedido es un chequeo de estado. No genera trazas,
// métricas ni logs: los chequeos de estado del balanceador y del Docker Compose
// lo van a consultar todo el tiempo, y solo agregarían ruido.
func isHealthCheck(r *http.Request) bool {
	return r.URL.Path == "/health"
}
