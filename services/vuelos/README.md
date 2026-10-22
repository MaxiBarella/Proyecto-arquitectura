# Servicio Vuelos

Organización interna: **CQRS** (decisión 7.3 de [docs/decisiones.md](../../docs/decisiones.md)).

El servicio arranca, se conecta a MySQL y responde `GET /health`. Ya tiene el vuelo con sus estados (`internal/domain`) y cómo se guarda (`internal/adapters/mysql`); faltan los endpoints y los eventos.

Se conecta a MySQL con la variable `MYSQL_DSN`, que arma el Docker Compose con el usuario propio de Vuelos, le pide los slots a Pistas en `PISTAS_URL` y publica y consume eventos en `RABBITMQ_URL`.

Tests: `go test ./...`. Los del adaptador de MySQL levantan un MySQL real con testcontainers y necesitan Docker abierto; sin Docker, se saltean.

| Carpeta | Para qué es |
|---|---|
| `cmd/api` | Punto de entrada: arma el servidor y lo pone a escuchar. |
| `internal/domain` | Contiene el vuelo, sus estados y las transiciones válidas. |
| `internal/commands` | Lado de escritura: publicar y cancelar vuelos, pedir slot. |
| `internal/queries` | Lado de lectura: el tablero y la búsqueda, que leen del índice. |
| `internal/adapters/http` | Expone comandos y consultas por HTTP con Gin. |
| `internal/adapters/mysql` | Guarda los vuelos, el outbox y los mensajes procesados en MySQL. |
| `db` | Las tablas de MySQL. La base las crea la primera vez que arranca; si cambian, hay que borrar el volumen con `docker compose down -v`. |
| `internal/adapters/rabbitmq` | Publica los eventos del outbox y consume `vuelos.eventos` (4.21 a 4.24). |
| `internal/observability` | Logs en JSON, trazas y métricas con OpenTelemetry. Es una copia de la carpeta de Clima: se copia, no se importa. |
| `internal/adapters/pistas` | Le pide los slots a Pistas, con timeout, reintentos y circuit breaker (4.5, 13.1). |
| `internal/adapters/solr` | Mantiene y consulta el índice de búsqueda en Solr. |

Puerto en el arranque local: `8081`.

Los endpoints están en [openapi.yaml](openapi.yaml).
