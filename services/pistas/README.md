# Servicio Pistas

Organización interna: **hexagonal** (decisión 7.3 de [docs/decisiones.md](../../docs/decisiones.md)).

Es la estructura inicial: el servicio arranca y responde `GET /health`. Todavía no tiene lógica.

| Carpeta | Para qué es |
|---|---|
| `cmd/api` | Punto de entrada: arma el servidor y lo pone a escuchar. |
| `internal/domain` | Contiene las reglas de asignación de slots y de cierre de pistas; no conoce ni la base ni el framework. |
| `internal/ports` | Declara las interfaces por las que el dominio se comunica con el exterior. |
| `internal/adapters/http` | Adaptador de entrada: expone el dominio por HTTP con Gin. |
| `internal/adapters/mysql` | Adaptador de salida: guarda pistas, slots y el outbox en MySQL. |
| `internal/adapters/rabbitmq` | Adaptador de mensajería: publica y consume eventos en RabbitMQ. |

Puerto en el arranque local: `8082`.
