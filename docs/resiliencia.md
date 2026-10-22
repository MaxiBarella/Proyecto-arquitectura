# Resiliencia: qué pasa cuando se cae Pistas

Evidencia de cómo el sistema gestiona la caída de su función más importante, pedir un slot. Es lo que pide el enunciado para la resiliencia y lo que justifica el [ADR-010](adr/ADR-010.md).

## La función y el mecanismo

**Pedir un slot** es la única llamada síncrona entre servicios propios: la aerolínea le pide un slot a Vuelos, y Vuelos se lo pide a Pistas (4.1). Si Pistas está caído, sin protección Vuelos haría esperar a cada aerolínea y le sumaría carga a Pistas justo cuando está mal.

Vuelos se protege con tres mecanismos, que se suman:

| Mecanismo | Valor | Decisión |
|---|---|---|
| Timeout de cada intento | 2 segundos | 4.5, 4.16 |
| Reintentos ante una falla pasajera | Hasta 2, con espera creciente desde 200 ms | 4.5 |
| Circuit breaker | Se abre con 5 pedidos fallidos seguidos, queda abierto 30 segundos y después deja pasar 1 pedido de prueba | 13.1, 13.2 |

Con el breaker abierto, Vuelos responde `503` con `Retry-After: 30` sin llamar a Pistas, y el vuelo queda en "pendiente de slot": la aerolínea lo puede volver a pedir.

## La prueba

El 22/10/2026 se levantaron Vuelos y Pistas con el Docker Compose y se apagó Pistas mientras una aerolínea pedía slots. Las horas son de Argentina.

**1. Con Pistas funcionando,** un pedido de slot responde `202` al instante.

**2. 16:57:58: se apaga Pistas** con `docker stop`.

**3. La aerolínea sigue pidiendo slots:**

| Pedido | Respuesta | Tardó | Estado del vuelo |
|---|---|---|---|
| 1 a 5 | `503 RUNWAYS_UNAVAILABLE`, `Retry-After: 30` | Unos 6 segundos cada uno: 3 intentos, cada uno con su timeout | Pendiente de slot |
| 6 y 7 | `503 RUNWAYS_UNAVAILABLE`, `Retry-After: 30` | **Al instante**: el breaker ya está abierto y Vuelos no llama a Pistas | Pendiente de slot |

**4. El resto de Vuelos sigue andando:** publicar vuelos y listarlos responde normal, sin Pistas.

**5. 16:58:29: se vuelve a prender Pistas.** Vuelos no lo llama hasta que pasan los 30 segundos de breaker abierto.

**6. Pistas volvió:** el pedido de prueba pasa, el breaker se cierra, y los pedidos siguientes responden `202`. Los vuelos quedan programados al llegar el evento "slot asignado".

## El log de Vuelos

Las líneas del circuit breaker, como las escribe Vuelos en JSON (resumidas; la hora es UTC, tres más que la de Argentina):

```text
19:58:00 WARN reintento del pedido de slot  {"attempt": 1, "error": "falla pasajera: Post \"http://pistas:8080/internal/slots\": context deadline exceeded"}
19:58:02 WARN reintento del pedido de slot  {"attempt": 2, "error": "falla pasajera: Post \"http://pistas:8080/internal/slots\": dial tcp: lookup pistas ..."}
   ... los mismos 2 reintentos en cada uno de los 5 primeros pedidos (10 en total) ...
19:58:29 WARN cambió el circuit breaker     {"from": "closed", "to": "open"}
19:58:29 WARN no se pudo pedir el slot      {"error": "pedir el slot a Pistas: Pistas no está disponible: el circuit breaker está abierto"}
19:58:29 WARN no se pudo pedir el slot      {"error": "pedir el slot a Pistas: Pistas no está disponible: el circuit breaker está abierto"}
19:59:01 WARN cambió el circuit breaker     {"from": "open", "to": "half-open"}
19:59:01 WARN cambió el circuit breaker     {"from": "half-open", "to": "closed"}
```

En Grafana (<http://localhost:3001>, Explore, Loki) se ven con `{service_name="vuelos"} |= "circuit breaker"`.

## Qué muestra

- **La caída no se propaga.** Con Pistas caído, Vuelos sigue publicando y mostrando vuelos. Lo único que no se puede es conseguir un slot, y el vuelo queda pendiente, sin perderse.
- **El breaker corta la espera.** Los primeros 5 pedidos tardan unos 6 segundos, por los reintentos; desde el sexto, la respuesta es inmediata y Pistas no recibe tráfico mientras se recupera.
- **La aerolínea sabe qué hacer.** Recibe un código claro (`RUNWAYS_UNAVAILABLE`) y cuándo reintentar (`Retry-After: 30`).
- **La recuperación es sola.** Cuando Pistas vuelve, el pedido de prueba cierra el breaker sin que nadie intervenga.

## Lo que no muestra

- Cada instancia de Vuelos tiene su propio breaker (12.1). La prueba usó una sola; con dos, cada una se abre por su cuenta.
- Mientras el breaker está abierto, no se piden slots aunque Pistas ya haya vuelto: en la prueba, Pistas respondía a las 16:58:29 y el breaker dejó pasar el pedido de prueba a las 16:59:01.

## Cómo repetirla

1. `docker compose up -d --build vuelos pistas`
2. Publicar un vuelo con `POST /flights` en Vuelos (puerto 8081), con los encabezados `X-User-Id`, `X-User-Role: airline` y `X-Airline`.
3. `docker compose stop pistas`
4. Pedir slots con `POST /flights/{id}/slot-request` al menos 6 veces: los 5 primeros tardan, el sexto responde al instante.
5. `docker compose start pistas`, esperar 30 segundos y volver a pedir: responde `202`.
6. `docker compose logs vuelos | grep "circuit breaker"`
