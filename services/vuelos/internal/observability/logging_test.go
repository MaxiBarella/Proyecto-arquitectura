package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestLogger arma el mismo logger que NewLogger, pero escribiendo en un buffer.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(traceHandler{Handler: slog.NewJSONHandler(buf, nil)}).With("service", "clima")
}

// decodeLines devuelve cada línea JSON del buffer como un mapa.
func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var line map[string]any
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("la línea no es JSON: %q", raw)
		}
		lines = append(lines, line)
	}
	return lines
}

func contextWithSpan() (context.Context, trace.SpanContext) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19},
		SpanID:     trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(context.Background(), sc), sc
}

func TestTraceHandler(t *testing.T) {
	ctx, sc := contextWithSpan()

	tests := []struct {
		name      string
		ctx       context.Context
		group     string
		wantTrace bool
	}{
		{name: "dentro de una traza", ctx: ctx, wantTrace: true},
		{name: "dentro de una traza, con un grupo", ctx: ctx, group: "datos", wantTrace: true},
		{name: "sin traza", ctx: context.Background(), wantTrace: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := newTestLogger(&buf)
			if tt.group != "" {
				logger = logger.WithGroup(tt.group)
			}
			logger.InfoContext(tt.ctx, "mensaje")

			lines := decodeLines(t, &buf)
			if len(lines) != 1 {
				t.Fatalf("se esperaba una línea, hubo %d", len(lines))
			}
			line := lines[0]
			if line["service"] != "clima" {
				t.Errorf("service = %v, se esperaba clima", line["service"])
			}

			fields := line
			if tt.group != "" {
				fields, _ = line[tt.group].(map[string]any)
			}
			gotTrace, hasTrace := fields["trace_id"]
			if hasTrace != tt.wantTrace {
				t.Fatalf("trace_id presente = %v, se esperaba %v (línea: %v)", hasTrace, tt.wantTrace, line)
			}
			if tt.wantTrace {
				if gotTrace != sc.TraceID().String() {
					t.Errorf("trace_id = %v, se esperaba %s", gotTrace, sc.TraceID())
				}
				if fields["span_id"] != sc.SpanID().String() {
					t.Errorf("span_id = %v, se esperaba %s", fields["span_id"], sc.SpanID())
				}
			}
		})
	}
}

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)
	router := gin.New()
	router.Use(RequestLogger(logger), Recovery(logger))
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/airports/:icao", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/explota", func(c *gin.Context) { panic("algo salió mal") })

	t.Run("no registra /health", func(t *testing.T) {
		buf.Reset()
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
		if buf.Len() != 0 {
			t.Fatalf("se esperaba ningún log, hubo: %s", buf.String())
		}
	})

	t.Run("registra la ruta, no la URL", func(t *testing.T) {
		buf.Reset()
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/airports/SACO", nil))
		line := decodeLines(t, &buf)[0]
		if line["route"] != "/airports/:icao" || line["status"] != float64(http.StatusOK) {
			t.Errorf("route = %v, status = %v", line["route"], line["status"])
		}
	})

	t.Run("sin ruta, registra la URL", func(t *testing.T) {
		buf.Reset()
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/no-existe", nil))
		line := decodeLines(t, &buf)[0]
		if line["route"] != "/no-existe" || line["status"] != float64(http.StatusNotFound) {
			t.Errorf("route = %v, status = %v", line["route"], line["status"])
		}
	})

	t.Run("un pánico queda en JSON y con su línea de pedido", func(t *testing.T) {
		buf.Reset()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/explota", nil))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, se esperaba 500", rec.Code)
		}
		lines := decodeLines(t, &buf)
		if len(lines) != 2 {
			t.Fatalf("se esperaban 2 líneas (el pánico y el pedido), hubo %d", len(lines))
		}
		if lines[0]["level"] != "ERROR" || lines[0]["error"] != "algo salió mal" {
			t.Errorf("línea del pánico: %v", lines[0])
		}
		if lines[1]["msg"] != "pedido HTTP" || lines[1]["status"] != float64(http.StatusInternalServerError) {
			t.Errorf("línea del pedido: %v", lines[1])
		}
	})
}
