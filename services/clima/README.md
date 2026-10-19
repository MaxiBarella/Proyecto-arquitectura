# Servicio Clima

Organización interna: **capas** (decisión 7.3 de [docs/decisiones.md](../../docs/decisiones.md)).

Consulta a Open-Meteo cada 15 minutos, guarda la observación en MongoDB y calcula si se puede operar en Córdoba. Responde la API pública del [contrato](../../docs/contracts/README.md) y, en el Docker Compose, deja en el outbox los eventos para Pistas. Las decisiones están en la sección 9 de [docs/decisiones.md](../../docs/decisiones.md).

| Ruta | Para qué |
|---|---|
| `GET /airports/SACO/conditions` | API pública: el último dato real y su aptitud. Pide `X-API-Key`. |
| `PUT /internal/airports/SACO/forced-conditions` | Fuerza valores de clima (9.12). Pide `X-User-Role: operator`. Solo en el Docker Compose. |
| `DELETE /internal/airports/SACO/forced-conditions` | Quita la condición forzada. |
| `GET /health` | Para el balanceador y los chequeos. |

| Variable | Qué es |
|---|---|
| `MONGO_URI` | Conexión a MongoDB. Obligatoria. |
| `CLIMA_API_KEYS` | Claves de los consumidores, como `grupo1:clave1,grupo2:clave2`. Obligatoria. |
| `RABBITMQ_ENABLED` | `false` en la copia de la nube: no encola eventos ni expone el forzado. Por defecto `true`. |
| `RABBITMQ_URL` | Conexión a RabbitMQ, si está habilitado. |
| `PORT` | Puerto. Por defecto `8080`. |

Tests: `go test ./...` desde esta carpeta.

| Carpeta | Para qué es |
|---|---|
| `cmd/api` | Punto de entrada: arma el servidor y lo pone a escuchar. |
| `internal/controllers` | Recibe los pedidos HTTP y arma las respuestas. |
| `internal/services` | Contiene la lógica: condiciones actuales y aptitud operativa. |
| `internal/repositories` | Lee y guarda en MongoDB, y consulta a Open-Meteo. |
| `internal/models` | Define los datos que maneja el servicio. |
| `internal/observability` | Logs en JSON, métricas y trazas con OpenTelemetry. Ver su [README](internal/observability/README.md). |

Puerto en el arranque local: `8083`.
