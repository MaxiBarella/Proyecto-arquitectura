# Pruebas de carga

Los escenarios del [ADR-013](../../docs/adr/ADR-013.md), escritos para k6 (16.1). Se corren con Docker, desde el compose, así nadie tiene que instalar k6 (16.4). Todos entran por NGINX, como un usuario real, y mandan sus métricas a Grafana (16.5).

| Escenario | Archivo | Qué hace | Cuánto dura |
|---|---|---|---|
| 1. Tablero | `tablero.js` | Rampa de 10 a 1000 pedidos por segundo al tablero público (16.2). Se corre sin caché y con caché. | 5 min |
| Datos del tablero | `sembrar-tablero.js` | Publica 700 vuelos en los próximos 7 días y les pide slot a 3 de cada 4 (10.9). Se corre una vez antes del escenario 1. | 1,5 min |
| 2. Pedido de slot | `slot.js` | Publica vuelos y pide slots distintos, a 5, 20 y 50 por segundo (16.3). Mide el objetivo de servicio (11.3). | 3 min |
| 3. Mismo slot a la vez | `mismo-slot.js` | 50 vuelos piden la misma pista y el mismo bloque a la vez. Lo tiene que obtener uno solo (RN-10). | segundos |
| 4. Caída durante la carga | `caida.js` y `caida.sh` | 100 pedidos por segundo durante 3 minutos. Apaga `vuelos-2` al minuto y la prende al segundo (ADR-012). | 3 min |

## Cómo se corre

1. **Levantar el sistema con el límite del gateway subido.** Con el límite normal (20 por segundo, 15.8) casi todo da `429`:

   ```sh
   GATEWAY_RATE_LIMIT=100000 GATEWAY_RATE_BURST=100000 docker compose up -d --build
   ```

   Al terminar, `docker compose up -d nginx` lo vuelve al valor normal.

2. **Correr un escenario.** El resumen sale en pantalla y queda en `tests/carga/resultados/`:

   ```sh
   docker compose --profile carga run --rm k6 run --summary-export /resultados/slot.json /carga/slot.js
   docker compose --profile carga run --rm k6 run --summary-export /resultados/mismo-slot.json /carga/mismo-slot.js
   docker compose --profile carga run --rm k6 run --summary-export /resultados/tablero.json /carga/tablero.js
   tests/carga/caida.sh
   ```

   En Git Bash de Windows hay que poner `MSYS_NO_PATHCONV=1` adelante de cada `docker compose run`, por ejemplo `MSYS_NO_PATHCONV=1 docker compose --profile carga run --rm k6 run /carga/slot.js`. Si no, Git Bash convierte `/carga` en una ruta de Windows y k6 no encuentra el archivo. `caida.sh` ya la pone.

   - **El tablero** se corre dos veces, sin caché y con caché, sobre los mismos datos (ADR-007):
     1. Una sola vez, cargar los vuelos: `docker compose --profile carga run --rm k6 run /carga/sembrar-tablero.js`.
     2. Sin caché: `CACHE_ENABLED=false` en el `.env`, `docker compose up -d vuelos-1 vuelos-2` y `docker compose restart memcached`. Correr `tablero.js` con `--summary-export /resultados/tablero-sin-cache.json`.
     3. Con caché: sacar `CACHE_ENABLED` del `.env`, repetir los dos comandos del paso anterior y correr `tablero.js` con `--summary-export /resultados/tablero-con-cache.json`.
     4. Los aciertos de la caché se leen de Memcached al terminar: `docker run --rm --network <proyecto>_default busybox:1.36 sh -c 'printf "stats\r\nquit\r\n" | nc memcached 11211' | grep get_`.

     La mezcla de consultas se cambia con `-e PORCENTAJE_PORTADA=80`.
   - **La caída** se puede correr contra otra ruta de Vuelos con `RUTA=/api/vuelos/flights tests/carga/caida.sh`.

3. **Mirar Grafana** (`http://localhost:3001`, Explore, Prometheus) mientras corre:

   | Qué | Consulta |
   |---|---|
   | Pedidos por segundo que manda k6 | `sum(rate(http_reqs_total{service_name="k6"}[1m]))` |
   | Reparto entre las instancias de Vuelos | `sum by (instance) (rate(http_server_request_duration_seconds_count{service_name="vuelos"}[2m]))` |

   Los servicios mandan sus métricas una vez por minuto, así que conviene ventanas de 2 minutos o más.

4. **CPU y memoria de cada contenedor:** `docker stats` en otra terminal (ADR-013).

Los resultados (`resultados/*.json`) no se suben al repo: dependen de la computadora donde se corrieron. Los números que valen quedan anotados en el ADR-013.
