# Servicio Clima

Organización interna: **capas** (decisión 7.3 de [docs/decisiones.md](../../docs/decisiones.md)).

Por ahora el servicio arranca, responde `GET /health` y envía sus logs, métricas y trazas a Grafana. Todavía no tiene lógica de negocio.

| Carpeta | Para qué es |
|---|---|
| `cmd/api` | Punto de entrada: arma el servidor y lo pone a escuchar. |
| `internal/controllers` | Recibe los pedidos HTTP y arma las respuestas. |
| `internal/services` | Contiene la lógica: condiciones actuales y aptitud operativa. |
| `internal/repositories` | Lee y guarda las observaciones en MongoDB. |
| `internal/models` | Define los datos que maneja el servicio. |
| `internal/observability` | Logs en JSON, métricas y trazas con OpenTelemetry. Ver su [README](internal/observability/README.md). |

Puerto en el arranque local: `8083`.
