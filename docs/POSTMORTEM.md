# Postmortem: caída provocada de Pistas y de RabbitMQ

- **Fecha del ensayo:** 9/11/2026, de 15:50:54 a 15:56:06 (hora de Argentina)
- **Tipo:** caída controlada, hecha a propósito como ensayo (enunciado 2.9; decisión 24.1)
- **Entorno:** el Docker Compose en una máquina local, con una instancia de Vuelos. Pistas corría con el código del cierre por clima (rama `feat/pistas-clima`).
- **Quién la hizo:** Carola, con el agente de IA

## Resumen

Se tiraron abajo, de a una, dos dependencias del flujo principal: primero **Pistas** y después **RabbitMQ**. En ninguno de los dos casos se perdió un pedido ni un evento, y el sistema volvió solo a la normalidad cuando volvió la dependencia.

- **Pistas caída:** los primeros 5 pedidos de slot tardaron unos 6,5 segundos y fallaron. Después se abrió el circuit breaker de Vuelos y los pedidos fallaron en 0,1 segundos, sin llamar a Pistas. 26 vuelos quedaron "pendiente de slot". El sistema se recuperó 28 segundos después de que volvió Pistas.
- **RabbitMQ caído:** los pedidos de slot siguieron funcionando, pero los vuelos quedaron "pendiente de slot" porque el evento "slot asignado" no podía salir. El outbox de Pistas guardó los eventos y los publicó 16 segundos después de que volvió RabbitMQ. Los 29 vuelos de ese rato quedaron programados.

## Cómo se hizo

Un script mandó un pedido de slot cada 2 o 3 segundos durante todo el ensayo: publicaba un despegue de `AR` y le pedía un slot libre en la API de Vuelos. En total fueron 100 pedidos, de `AR5001` a `AR5100`. Cada respuesta quedó registrada con su hora, su código y lo que tardó. Las dependencias se apagaron y prendieron con `docker compose stop` y `docker compose start`.

Lo que pasaba se miró en Grafana (Explore), con tres consultas:

- **Prometheus:** respuestas de `POST /flights/:id/slot-request` en Vuelos, por código.
- **Prometheus:** el tiempo de respuesta del mismo pedido (percentil 95).
- **Loki:** los logs de Vuelos con "circuit breaker".

## Línea temporal

| Hora | Qué pasó |
|---|---|
| 15:50:54 | Arranca la carga. Cada pedido responde `202` en 0,1 segundos y el vuelo pasa a programado al llegar "slot asignado". |
| **15:52:16** | **Se apaga Pistas.** |
| 15:52:26 a 15:53:02 | 5 pedidos seguidos (`AR5032` a `AR5036`) tardan entre 6,3 y 6,8 segundos: Vuelos intenta 3 veces, con sus esperas, y responde `503 RUNWAYS_UNAVAILABLE`. |
| **15:53:02** | **Se abre el circuit breaker** de Vuelos a Pistas, a la quinta falla seguida. Desde acá los pedidos responden `503` en 0,1 segundos. |
| 15:53:32 | Pasan los 30 segundos: el circuit breaker deja pasar un pedido de prueba. Pistas sigue caída, falla, y a las 15:53:39 se vuelve a abrir. |
| **15:53:41** | **Se prende Pistas.** |
| **15:54:09** | El siguiente pedido de prueba anda y **se cierra el circuit breaker**. `AR5058` es el primero que vuelve a quedar programado. |
| **15:54:47** | **Se apaga RabbitMQ.** Los pedidos siguen respondiendo `202` en 0,1 segundos. |
| 15:54:47 a 15:55:50 | Pistas asigna los slots y guarda cada "slot asignado" en su outbox, pero no lo puede publicar: llegó a tener más de 20 eventos sin publicar. Los vuelos quedan "pendiente de slot". |
| **15:55:50** | **Se prende RabbitMQ.** |
| **15:56:06** | El outbox de Pistas queda vacío. Los 29 vuelos de ese rato (`AR5072` a `AR5100`) pasan a programado. |

## Impacto

| | Pistas caída | RabbitMQ caído |
|---|---|---|
| Duración de la caída | 1 min 25 s (15:52:16 a 15:53:41) | 1 min 3 s (15:54:47 a 15:55:50) |
| Duración del impacto | 1 min 53 s (15:52:16 a 15:54:09) | 1 min 19 s (15:54:47 a 15:56:06) |
| Qué vio la aerolínea | Su pedido de slot fue rechazado con `503`: los 5 primeros después de esperar unos 6,5 s, los demás enseguida. | Su pedido fue aceptado, pero el vuelo tardó en pasar a programado: hasta 1 min 19 s, en lugar de unos 300 ms. |
| Vuelos afectados | 26 (`AR5032` a `AR5057`) quedaron "pendiente de slot". La aerolínea tiene que volver a pedir. | 29 (`AR5072` a `AR5100`) quedaron programados cuando volvió RabbitMQ, sin hacer nada. |
| Datos perdidos | Ninguno. | Ninguno. La cola de mensajes fallidos quedó vacía. |
| Lo que siguió andando | Publicar vuelos: los 100 se publicaron bien. Cancelar y el tablero no se probaron. | Publicar vuelos y pedir slots: solo se demoraron los cambios de estado. |

## Detección

- **El circuit breaker quedó en los logs de Vuelos** como WARN, con el estado anterior y el nuevo (`closed` → `open`, `open` → `half-open`, `half-open` → `closed`). En Loki se ve cada cambio con su hora.
- **En las métricas** se ve el pedido de slot pasar de `202` a `503`, y el tiempo de respuesta subir a unos 6,5 segundos y después bajar casi a cero cuando se abre el circuit breaker.
- **RabbitMQ caído** se ve en los logs: Pistas registró 28 veces "no se pudo publicar el evento; se reintenta en la próxima vuelta", y los consumidores, 22 veces "se cortó el consumo de eventos; se vuelve a conectar".
- **No saltó ninguna alerta.** Las dos que hay (mensajes en fallidos, y el p95 del pedido de slot arriba de 500 ms durante 5 minutos) no aplican a una caída de menos de 2 minutos sin mensajes fallidos. La caída se detectó mirando.

## Respuesta

No hubo que hacer nada a mano para recuperarse: cuando volvió cada dependencia, el sistema se normalizó solo.

- **Pistas:** el circuit breaker probó a los 30 segundos, encontró a Pistas de vuelta y se cerró (13.2).
- **RabbitMQ:** los consumidores se reconectaron solos y el relay publicó todo lo pendiente (4.4, 4.5).

## Causa

Fue provocada: se apagaron los contenedores a propósito (24.1). Lo que interesa es por qué el sistema se comportó así:

- **Pistas caída.** Vuelos llama a Pistas de forma síncrona para pedir el slot (4.1). Cada llamada tiene 2 segundos de timeout y hasta 2 reintentos (4.5, 4.16), y por eso cada pedido tardó unos 6,5 segundos hasta que el circuit breaker se abrió con 5 fallas seguidas (13.2). Mientras está abierto, Vuelos no llama a Pistas y responde enseguida: la caída de Pistas no deja a las aerolíneas esperando ni le carga más pedidos a Pistas cuando vuelve.
- **RabbitMQ caído.** "Slot asignado" se guarda en el outbox de Pistas en la misma transacción que el slot (4.4). El relay lo intenta publicar cada segundo y lo saca del outbox solo cuando RabbitMQ lo confirma. Por eso no se perdió ningún evento.

## Lo que se confirmó

- El circuit breaker con los valores de la 13.2: 5 fallas seguidas, 30 segundos abierto, 1 pedido de prueba (ADR-010).
- El tiempo que espera la aerolínea con Pistas caída: unos 6,5 segundos, como calculaba el ADR-005 ("unos 7 segundos").
- El outbox no pierde eventos con RabbitMQ caído, y los publica cuando vuelve (ADR-005).
- Los consumidores se reconectan solos y nada termina en fallidos.

## Acciones de mejora

Son propuestas del ensayo. Ninguna está decidida: se llevan al grupo.

| # | Qué se vio | Propuesta |
|---|---|---|
| 1 | Una caída de Pistas no dispara ninguna alerta. | Sumar una alerta cuando el circuit breaker de Vuelos se abre (el log WARN "cambió el circuit breaker" con `to: open`), parecida a la de mensajes fallidos (11.4). |
| 2 | Los 26 vuelos rechazados quedan "pendiente de slot" y la aerolínea tiene que volver a pedir a mano. | Que la pantalla de la aerolínea muestre el `503` con un mensaje claro ("Pistas no está disponible, probá en un momento") y deje reintentar fácil. |
| 3 | Después de que vuelve Pistas, el sistema tarda hasta 30 segundos más en usarla, por el tiempo que el circuit breaker queda abierto. | Se acepta: es el precio de no cargar a Pistas apenas vuelve. Si en la presentación molesta, revisar el valor con las pruebas de carga (ADR-013). |
| 4 | Con RabbitMQ caído, Pistas y los consumidores escriben un error por segundo. | Se acepta para el alcance del trabajo. Mejora posible: espaciar los reintentos del relay mientras RabbitMQ no responde. |
| 5 | El ensayo se hizo con una sola instancia de Vuelos y sin el frontend, que todavía no estaban mergeados. | Repetirlo antes de la presentación con las dos instancias y el frontend, para ver la caída también en la pantalla de la aerolínea (24.1). |

## Evidencia

- Registro de los 100 pedidos, con hora, código y duración: en la máquina donde se hizo el ensayo.
- Logs de Vuelos y de Pistas en Loki, entre las 15:50 y las 15:57 del 9/11/2026.
- Capturas de Grafana: *(pendiente: sumarlas al pull request)*.
