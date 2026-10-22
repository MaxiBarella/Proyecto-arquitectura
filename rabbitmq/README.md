# RabbitMQ

`definitions.json` crea el exchange de eventos, la cola de cada consumidor, sus uniones y sus colas de mensajes fallidos (decisiones 4.13 y 4.17 a 4.20 de [docs/decisiones.md](../docs/decisiones.md)). Así ningún evento se pierde por publicarse antes de que arranque el servicio que lo recibe.

| Cola | Consumidor | Recibe | Fallidos en |
|---|---|---|---|
| `pistas.eventos` | Pistas | `clima.#` y `vuelos.vuelo.cancelado` | `pistas.eventos.fallidos` |
| `vuelos.eventos` | Vuelos | `pistas.#` | `vuelos.eventos.fallidos` |

**Cómo se carga.** RabbitMQ arranca con el usuario del `.env`. Cuando está listo, el contenedor `rabbitmq-init` del Docker Compose sube este archivo por la API de administración, una sola vez, y termina. Los servicios arrancan recién después.

No se usa `load_definitions` al arrancar porque, si RabbitMQ carga un archivo así, no crea el usuario del `.env`, y poner el usuario en el archivo dejaría la contraseña en el repositorio.

**Mensajes fallidos.** Un consumidor que no puede procesar un mensaje después de sus reintentos (4.5) lo rechaza sin volver a encolarlo (`nack` con `requeue=false`). RabbitMQ lo pasa solo, por el exchange `aeropuerto.fallidos`, a la cola de fallidos de ese consumidor (11.4).

**Sumar un consumidor o un evento.** Se agrega la cola, su cola de fallidos y las uniones en este archivo, y se reinicia con `docker compose up rabbitmq-init`. Importar el archivo de nuevo no borra nada de lo que ya existe.
