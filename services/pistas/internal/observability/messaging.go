package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Traza en los eventos (decisión 11.6). El evento guarda en el outbox el
// traceparent de la traza en curso; el relay lo usa para publicar dentro de esa
// misma traza y lo pone como encabezado del mensaje; el consumidor lo lee para
// seguirla. Así Grafana muestra el flujo entero como una sola traza.

const tracerName = "aeropuerto/mensajeria"

// traceContext es el formato estándar de W3C, el mismo que viaja en HTTP.
var traceContext = propagation.TraceContext{}

// TraceParent devuelve el encabezado traceparent de la traza en curso, para
// guardarlo en el outbox junto con el evento. Si no hay traza, devuelve "".
func TraceParent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	traceContext.Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

// TraceID devuelve el identificador de la traza en curso, que va en el
// correlation_id de los eventos (11.2). Si no hay traza, devuelve "".
func TraceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

// StartPublish abre el tramo de publicar un evento. Si el evento guardó un
// traceparent, el tramo queda dentro de esa traza; si no (por ejemplo, un evento
// de una tarea periódica), empieza una traza nueva. Hay que cerrarlo con End.
func StartPublish(ctx context.Context, traceparent, eventType string) (context.Context, trace.Span) {
	if traceparent != "" {
		ctx = traceContext.Extract(ctx, propagation.MapCarrier{"traceparent": traceparent})
	}
	return otel.Tracer(tracerName).Start(ctx, "publicar "+eventType,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", eventType),
		))
}

// MessageHeaders devuelve los encabezados para el mensaje de RabbitMQ con la
// traza en curso. amqp.Table es un map[string]interface{}, así que se puede
// pasar directo.
func MessageHeaders(ctx context.Context) map[string]interface{} {
	headers := map[string]interface{}{}
	if tp := TraceParent(ctx); tp != "" {
		headers["traceparent"] = tp
	}
	return headers
}

// StartConsume abre el tramo de procesar un mensaje recibido, dentro de la
// traza que trae su encabezado traceparent. Lo usa cada consumidor. Hay que
// cerrarlo con End.
func StartConsume(ctx context.Context, headers map[string]interface{}, eventType string) (context.Context, trace.Span) {
	if tp, ok := headers["traceparent"].(string); ok && tp != "" {
		ctx = traceContext.Extract(ctx, propagation.MapCarrier{"traceparent": tp})
	}
	return otel.Tracer(tracerName).Start(ctx, "procesar "+eventType,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", eventType),
		))
}

// EndSpan cierra un tramo y, si hubo un error, lo marca como fallido.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprint(err))
	}
	span.End()
}
