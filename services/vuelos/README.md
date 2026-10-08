# Servicio Vuelos

Organización interna: **CQRS** (decisión 7.3 de [docs/decisiones.md](../../docs/decisiones.md)).

Es la estructura inicial: el servicio arranca y responde `GET /health`. Todavía no tiene lógica.

| Carpeta | Para qué es |
|---|---|
| `cmd/api` | Punto de entrada: arma el servidor y lo pone a escuchar. |
| `internal/domain` | Contiene el vuelo, sus estados y las transiciones válidas. |
| `internal/commands` | Lado de escritura: publicar y cancelar vuelos, pedir slot. |
| `internal/queries` | Lado de lectura: el tablero y la búsqueda, que leen del índice. |
| `internal/adapters/http` | Expone comandos y consultas por HTTP con Gin. |
| `internal/adapters/mysql` | Guarda los vuelos y el outbox en MySQL. |
| `internal/adapters/rabbitmq` | Publica y consume eventos en RabbitMQ. |
| `internal/adapters/solr` | Mantiene y consulta el índice de búsqueda en Solr. |

Puerto en el arranque local: `8081`.
