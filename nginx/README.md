# NGINX: gateway y balanceador

Un solo NGINX hace las dos cosas, con un archivo para cada una (15.1, 15.3). Además sirve el frontend (15.7), que compila al construir la imagen (`Dockerfile`, desde la raíz del repo).

| Archivo | Qué hace |
|---|---|
| `gateway.conf.template` | Rutas, token y límite de tráfico (15). Al arrancar se convierte en `gateway.conf`. |
| `balanceo.conf` | Las dos instancias de Vuelos, `vuelos-1` y `vuelos-2` (12, ADR-012): reparte por turnos y, si una falla, la saca 10 segundos (12.4). |
| `nginx.conf` | Lo común: carga njs, deja pasar `JWT_SECRET` y arma el log en JSON. |
| `jwt.js` | El script que verifica el token (15.4). |
| `verifica-jwt-secret.sh` | No deja arrancar a NGINX si `JWT_SECRET` tiene menos de 32 bytes (15.21). |
| `token-de-prueba.sh` | Arma tokens de prueba mientras Usuarios no los emite (15.20). |
| `prueba/` | La prueba automática del gateway. |

## Qué hace con cada pedido

Escucha en `http://localhost:8080`.

| Ruta | Token | Va a |
|---|---|---|
| `POST /api/usuarios/login` | No | Usuarios, como `/login` |
| `GET /api/vuelos/board` | No | Vuelos, como `/board` |
| `/api/pistas/internal/...` | | `404`: es solo entre servicios (15.19) |
| `/api/vuelos/...`, `/api/pistas/...`, `/api/usuarios/...`, `/api/clima/...` | Sí | El servicio, sin el prefijo: `/api/vuelos/flights` llega como `/flights` |
| Cualquier otra de `/api/` | | `404` |
| Todo lo demás | No | El frontend (`frontend/`). Si no es un archivo, devuelve `index.html` y la pantalla la elige React Router |

**El token.** Va en `Authorization: Bearer <token>`. `jwt.js` verifica la firma HS256 con `JWT_SECRET` y el vencimiento, sin llamar a Usuarios (15.2). También rechaza un token con otro algoritmo, como `"alg": "none"`, un rol que no sea `airline` u `operator`, y una aerolínea sin código. Si algo falla, responde `401` con `INVALID_TOKEN` (15.18), y el log dice por qué en el campo `token`.

**La identidad.** Con un token válido, NGINX manda `X-User-Id` (`sub`), `X-User-Role` (`role`) y, si es una aerolínea, `X-Airline` (`airline`) (15.6). Siempre pisa lo que mande el cliente con esos nombres, aun en las rutas sin token.

**El límite.** 20 pedidos por segundo por IP, con ráfagas de hasta 40; al pasarlo, `429` con `RATE_LIMIT_EXCEEDED` (15.8). Se cambia con `GATEWAY_RATE_LIMIT` y `GATEWAY_RATE_BURST` en el `.env`.

**Un servicio caído.** NGINX responde `502` si no hay a quién mandarle el pedido, o `504` si no conectó o no respondió a tiempo, los dos con `SERVICE_UNAVAILABLE` y en Problem Details (15.23). El `502` se puede repetir, porque el pedido no llegó. El `504` avisa que el pedido pudo haberse aplicado: repetir un `POST /flights` puede duplicar el vuelo (12.8). Los errores que arma cada servicio pasan sin cambios.

## Cómo probarlo

Prueba automática, sin levantar el sistema: NGINX delante de un servicio falso que responde qué le llegó.

```sh
nginx/prueba/prueba.sh
```

Con el sistema levantado (`docker compose up --build`). Antes, poné en el `.env` una clave propia: `JWT_SECRET=` y lo que da `openssl rand -base64 32`. Con la del ejemplo NGINX no arranca (15.21).

```sh
TOKEN=$(nginx/token-de-prueba.sh airline AR)
curl -i localhost:8080/api/vuelos/flights                                   # 401
curl -i -H "Authorization: Bearer $TOKEN" localhost:8080/api/vuelos/flights # 200
```

`token-de-prueba.sh` toma la clave de `JWT_SECRET` o del `.env`.

## El balanceo

NGINX reparte los pedidos de Vuelos por turnos entre `vuelos-1` y `vuelos-2`. Para verlo:

```sh
docker compose logs nginx | grep '"upstream"'   # qué instancia atendió cada pedido (por IP)
docker compose stop vuelos-2                     # los pedidos siguen andando, todos a vuelos-1
docker compose start vuelos-2                    # a los 10 segundos vuelve a recibir pedidos
```

En Grafana (`http://localhost:3001`, Explore, Prometheus):

```
sum by (service_instance_id) (rate(http_server_request_duration_seconds_count{service_name="vuelos"}[1m]))
```
