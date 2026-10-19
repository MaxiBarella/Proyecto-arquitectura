# Observabilidad

Logs, métricas y trazas del servicio, según las decisiones 11.1, 11.2 y 11.5 de [docs/decisiones.md](../../../../docs/decisiones.md).

- **Logs:** una línea JSON por evento con `log/slog`, en la salida estándar y en Grafana (Loki). Las líneas escritas dentro de un pedido llevan `trace_id` y `span_id`.
- **Trazas:** una por cada pedido HTTP, en Grafana (Tempo). El contexto viaja entre servicios en el encabezado estándar `traceparent`.
- **Métricas:** `http.server.request.duration`, la duración de cada pedido por ruta y estado, en Grafana (Prometheus).

`/health` no genera trazas ni métricas, porque lo consultan el Docker Compose y el balanceador todo el tiempo.

Todo se envía por OpenTelemetry a la dirección de `OTEL_EXPORTER_OTLP_ENDPOINT`. Si la variable no está, el servicio funciona igual y solo escribe los logs en la salida estándar: es el caso de la copia de Clima en la nube (9.3).

## Dónde se ve

Con el Docker Compose levantado, Grafana está en <http://localhost:3001>. En **Explore**:

- **Tempo**, para las trazas: buscar por `service.name`.
- **Loki**, para los logs: `{service_name="clima"}`.
- **Prometheus**, para las métricas: `http_server_request_duration_seconds_count`.

## Cómo sumarlo a otro servicio

Cada servicio es un módulo de Go aparte, así que esta carpeta se copia, no se importa.

1. Copiar esta carpeta a `services/<servicio>/internal/observability/`.
2. Desde la carpeta del servicio, correr `go mod tidy` para que baje las librerías.
3. En `cmd/api/main.go`, en este orden (ver el de Clima como ejemplo):
   - `observability.Setup(ctx, serviceName)` al arrancar, y llamar a la función que devuelve al apagar;
   - `observability.NewLogger(serviceName)` y `slog.SetDefault(logger)`;
   - en Gin, `observability.Middleware(serviceName)` antes que `observability.RequestLogger(logger)`, para que el log lleve la traza.
4. En `docker-compose.yml`, sumarle al servicio `OTEL_EXPORTER_OTLP_ENDPOINT: http://grafana:4318` y `grafana` en `depends_on`.
5. Para escribir un log dentro de un pedido, usar `slog.InfoContext(c.Request.Context(), ...)`: con el contexto, la línea lleva la traza.

## Lo que falta

- **Llamadas entre servicios.** Para que la traza siga de Vuelos a Pistas, el cliente HTTP de Vuelos tiene que usar el transporte de `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` (`otelhttp.NewTransport`), que agrega el encabezado `traceparent`.
- **Eventos.** El identificador de la traza va en el campo de correlación de cada mensaje (11.2). Falta agregar acá las funciones que lo escriben al publicar y lo leen al consumir.
