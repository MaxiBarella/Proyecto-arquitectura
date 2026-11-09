package observability

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// recordSpans instala un proveedor de trazas que guarda los tramos en memoria.
func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	return recorder
}

func TestSinTrazaNoHayTraceParent(t *testing.T) {
	if tp := TraceParent(context.Background()); tp != "" {
		t.Errorf("sin traza tendría que ser vacío, fue %q", tp)
	}
	if id := TraceID(context.Background()); id != "" {
		t.Errorf("sin traza tendría que ser vacío, fue %q", id)
	}
	if h := MessageHeaders(context.Background()); len(h) != 0 {
		t.Errorf("sin traza no tendría que haber encabezados: %v", h)
	}
}

// El flujo completo: un pedido guarda el evento con su traceparent, el relay lo
// publica más tarde y el consumidor lo procesa. Los tres tramos son la misma traza.
func TestLaTrazaSigueDelPedidoAlConsumidor(t *testing.T) {
	recorder := recordSpans(t)

	reqCtx, reqSpan := otel.Tracer("prueba").Start(context.Background(), "pedido HTTP")
	saved := TraceParent(reqCtx) // lo que se guarda en el outbox
	traceID := TraceID(reqCtx)
	reqSpan.End()
	if saved == "" || traceID == "" {
		t.Fatal("con un tramo abierto tendría que haber traceparent y traceID")
	}

	// El relay corre en otra goroutine, sin la traza del pedido.
	pubCtx, pubSpan := StartPublish(context.Background(), saved, "clima.aptitud.cambio")
	headers := MessageHeaders(pubCtx)
	EndSpan(pubSpan, nil)

	_, conSpan := StartConsume(context.Background(), headers, "clima.aptitud.cambio")
	EndSpan(conSpan, errors.New("no se pudo aplicar"))

	spans := recorder.Ended()
	if len(spans) != 3 {
		t.Fatalf("se esperaban 3 tramos, hubo %d", len(spans))
	}
	for _, s := range spans {
		if s.SpanContext().TraceID().String() != traceID {
			t.Errorf("el tramo %q está en otra traza", s.Name())
		}
	}
	pub, con := spans[1], spans[2]
	if pub.Parent().SpanID() != reqSpan.SpanContext().SpanID() || pub.SpanKind() != trace.SpanKindProducer {
		t.Errorf("publicar tendría que colgar del pedido y ser de tipo productor")
	}
	if con.Parent().SpanID() != pub.SpanContext().SpanID() || con.SpanKind() != trace.SpanKindConsumer {
		t.Errorf("procesar tendría que colgar de publicar y ser de tipo consumidor")
	}
	if con.Status().Code.String() != "Error" {
		t.Errorf("el tramo con error tendría que quedar marcado: %v", con.Status())
	}
}

func TestUnEventoSinTrazaEmpiezaUnaNueva(t *testing.T) {
	recorder := recordSpans(t)
	pubCtx, span := StartPublish(context.Background(), "", "clima.sin_datos")
	EndSpan(span, nil)
	if TraceID(pubCtx) == "" || recorder.Ended()[0].Parent().IsValid() {
		t.Error("tendría que empezar una traza nueva, sin padre")
	}
	if _, ok := MessageHeaders(pubCtx)["traceparent"]; !ok {
		t.Error("el mensaje tendría que llevar la traza nueva")
	}
}
