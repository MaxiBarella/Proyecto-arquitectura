package observability

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMiddlewareNoTrazaHealth(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	router := gin.New()
	router.Use(Middleware("clima"))
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/airports/:icao", func(c *gin.Context) { c.Status(http.StatusOK) })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	if spans := recorder.Ended(); len(spans) != 0 {
		t.Fatalf("/health generó %d trazas, se esperaba ninguna", len(spans))
	}

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/airports/SACO", nil))
	if spans := recorder.Ended(); len(spans) != 1 {
		t.Fatalf("/airports/SACO generó %d trazas, se esperaba una", len(spans))
	}
}

func TestSinColector(t *testing.T) {
	t.Setenv(endpointEnv, "")
	previous := otel.GetTracerProvider()

	shutdown, err := Setup(context.Background(), "clima")
	if err != nil {
		t.Fatalf("Setup devolvió un error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("el apagado devolvió un error: %v", err)
	}
	if otel.GetTracerProvider() != previous {
		t.Error("sin colector no se tiene que registrar un proveedor de trazas")
	}
	if _, ok := NewLogger("clima").Handler().(traceHandler); !ok {
		t.Error("sin colector el logger tiene que escribir solo en la salida estándar")
	}
}

func TestConColector(t *testing.T) {
	t.Setenv(endpointEnv, "http://localhost:4318")

	if _, ok := NewLogger("clima").Handler().(*slog.MultiHandler); !ok {
		t.Error("con colector el logger tiene que escribir en la salida estándar y en Grafana")
	}
}
