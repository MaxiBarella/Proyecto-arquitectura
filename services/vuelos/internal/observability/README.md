# Observabilidad

Logs, métricas y trazas del servicio, según las decisiones 11.1, 11.2 y 11.5 de [docs/decisiones.md](../../../../docs/decisiones.md).

- **Logs:** una línea JSON por evento con `log/slog`, en la salida estándar y en Grafana (Loki). Las líneas escritas dentro de un pedido llevan `trace_id` y `span_id`.
- **Trazas:** una por cada pedido HTTP, en Grafana (Tempo). El contexto viaja entre servicios en el encabezado estándar `traceparent`.
- **Métricas:** `http.server.request.duration`, la duración de cada pedido por ruta y estado, en Grafana (Prometheus).

`/health` no genera trazas, métricas ni logs. Lo van a consultar todo el tiempo el balanceador y los chequeos de estado del Docker Compose, y solo agregaría ruido.

Si un handler entra en pánico, el servicio responde `500` y el error queda en el log como JSON, con la traza y el stack.

Todo se envía por OpenTelemetry a la dirección de `OTEL_EXPORTER_OTLP_ENDPOINT`. Si la variable no está, el servicio funciona igual y solo escribe los logs en la salida estándar: es el caso de la copia de Clima en la nube (9.3).

## Dónde se ve

Con el Docker Compose levantado, Grafana está en <http://localhost:3001>. En **Explore**:

- **Tempo**, para las trazas: buscar por `service.name`.
- **Loki**, para los logs: `{service_name="clima"}`.
- **Prometheus**, para las métricas: `http_server_request_duration_seconds_count`.

## Cómo sumarlo a otro servicio

Va en los cuatro servicios: Vuelos y Pistas en la Entrega 2, y Usuarios en noviembre. Cada servicio es un módulo de Go aparte, así que esta carpeta se copia, no se importa. Los tests vienen con la carpeta: `go test ./internal/observability/` verifica que la copia funcione.

1. Copiar esta carpeta a `services/<servicio>/internal/observability/`.
2. Desde la carpeta del servicio, correr `go mod tidy` para que baje las librerías.
3. En `cmd/api/main.go`, en este orden (ver el de Clima como ejemplo):
   - `observability.NewLogger(serviceName)` y `slog.SetDefault(logger)`, antes que nada, para que hasta un error de arranque salga en JSON;
   - `observability.Setup(ctx, serviceName)`, y llamar a la función que devuelve al salir, por cualquier camino;
   - en Gin, en este orden: `observability.Middleware(serviceName)`, `observability.RequestLogger(logger)` y `observability.Recovery(logger)`, en lugar de `gin.Default()` o `gin.Recovery()`.
4. En `docker-compose.yml`, sumarle al servicio `OTEL_EXPORTER_OTLP_ENDPOINT: http://grafana:4318` y `grafana` en `depends_on`.
5. Para escribir un log dentro de un pedido, usar `slog.InfoContext(c.Request.Context(), ...)`: con el contexto, la línea lleva la traza.

## Lo que falta

Las dos piezas van en un pull request aparte, esta semana, para que Vuelos y Pistas usen las mismas.

- **Llamadas entre servicios.** Para que la traza siga de Vuelos a Pistas, el cliente HTTP de Vuelos tiene que usar el transporte de `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` (`otelhttp.NewTransport`), que agrega el encabezado `traceparent`.
- **Eventos.** El identificador de la traza va en el campo de correlación de cada mensaje (11.2). Falta agregar acá las funciones que lo escriben al publicar y lo leen al consumir.
