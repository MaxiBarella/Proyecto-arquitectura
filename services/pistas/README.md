# Servicio Pistas

Organización interna: **hexagonal** (decisión 7.3 de [docs/decisiones.md](../../docs/decisiones.md)).

Asigna slots de pista sin que se pisen: un slot por vuelo, prioridad de los arribos sobre los despegues y pedidos simultáneos. Los endpoints están en [docs/contracts/pistas.yaml](../../docs/contracts/pistas.yaml) y los eventos que publica, en [docs/contracts/pistas-eventos.md](../../docs/contracts/pistas-eventos.md).

| Carpeta | Para qué es |
|---|---|
| `cmd/api` | Punto de entrada: lee la configuración, conecta los adaptadores con el núcleo y levanta el servidor y el relay. |
| `internal/domain` | Las reglas: validez de un bloque, prioridad de los arribos y slots libres más cercanos. No conoce ni la base ni el framework. |
| `internal/aplicacion` | Los casos de uso (asignar un slot, listar slots y pistas) y el relay del outbox. Coordina las reglas con los puertos. |
| `internal/ports` | Las interfaces por las que el núcleo habla con el exterior: repositorio, outbox y publicador. |
| `internal/adapters/http` | Adaptador de entrada: expone los casos de uso por HTTP con Gin. |
| `internal/adapters/mysql` | Adaptador de salida: guarda pistas, slots y el outbox en MySQL con GORM. |
| `internal/adapters/rabbitmq` | Adaptador de salida: publica los eventos en RabbitMQ. |

`internal/aplicacion` se sumó al programar: los casos de uso usan las interfaces de `ports`, y `ports` usa los tipos de `domain`, así que no podían ir en ninguno de los dos sin crear un import circular.

## Cómo garantiza "un slot, un solo vuelo"

1. La tabla `slots` tiene dos restricciones únicas: pista más bloque, y vuelo. La base no deja guardar dos vuelos en el mismo slot ni dos slots para el mismo vuelo (RN-04, 4.6).
2. Antes de decidir, se lee quién ocupa el slot con `SELECT ... FOR UPDATE`, así nadie lo cambia mientras se decide.
3. Si dos pedidos llegan a la vez por un slot libre, uno de los dos choca al insertar. Ese pedido se vuelve a correr, ve lo que guardó el otro y decide con eso: si es un arribo contra un despegue, se lo queda (RN-11); si no, se rechaza.
4. El cambio y su evento se guardan en la misma transacción (outbox, 4.4). Un relay los publica cada 1 segundo.

## Variables de entorno

| Variable | Ejemplo |
|---|---|
| `PORT` | `8080` |
| `MYSQL_DSN` | `root:clave@tcp(mysql-pistas:3306)/pistas?parseTime=true&loc=UTC` |
| `RABBITMQ_URL` | `amqp://usuario:clave@rabbitmq:5672/` |

Si RabbitMQ no está, el servicio arranca igual y atiende pedidos; los eventos esperan en el outbox hasta que se puedan publicar.

## Tests

```sh
go test ./...                                          # reglas, casos de uso y HTTP
go test -tags integration ./internal/adapters/mysql/   # pedidos simultáneos contra MySQL real; necesita Docker
```

Puerto en el arranque local: `8082`.

El contrato del pedido de slot que hace Vuelos está en [openapi.yaml](openapi.yaml) (4.15).
