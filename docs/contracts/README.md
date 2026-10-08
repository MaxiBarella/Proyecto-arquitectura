# API de Clima

Capacidad que publicamos para otros grupos: las **condiciones meteorológicas actuales** del aeropuerto de Córdoba y si se puede **operar** en él (aterrizar y despegar).

Este documento alcanza para integrarse sin hablar con nosotros. El contrato formal es [clima-v1.yaml](clima-v1.yaml) (OpenAPI 3.1); si algo de acá no coincide con el contrato, manda el contrato.

| | |
|---|---|
| **Versión vigente** | `v1` (contrato `1.0.0`) |
| **Estado** | Contrato y mock disponibles. La implementación real se publica en la nube para la Entrega 2. |
| **Formato** | JSON. Errores en `application/problem+json`. |
| **Autenticación** | Una clave por grupo consumidor, en el header `X-API-Key`. |
| **Límite** | 60 pedidos por minuto por clave. |

## Operación

### `GET /v1/airports/{icao}/conditions`

Devuelve la última observación del aeropuerto y su aptitud operativa.

**Qué se envía**

| Dónde | Nombre | Obligatorio | Descripción |
|---|---|---|---|
| Ruta | `icao` | Sí | Código ICAO del aeropuerto, cuatro letras mayúsculas. Hoy se atiende solo **`SACO`** (Córdoba). |
| Header | `X-API-Key` | Sí | La clave de tu grupo. |

**Qué se obtiene** (`200 OK`)

```json
{
  "airport": {
    "icao": "SACO",
    "name": "Aeropuerto Internacional Ingeniero Aeronáutico Ambrosio Taravella"
  },
  "observed_at": "2026-10-09T14:00:00Z",
  "conditions": {
    "temperature_c": 22.4,
    "wind_speed_kmh": 18.0,
    "wind_gust_kmh": 27.5,
    "wind_direction_deg": 340,
    "visibility_m": 10000,
    "precipitation_mm": 0.0,
    "thunderstorm": false
  },
  "operability": {
    "operable": true,
    "reasons": []
  }
}
```

| Campo | Tipo | Descripción |
|---|---|---|
| `airport.icao` | texto | Código ICAO consultado. |
| `airport.name` | texto | Nombre del aeropuerto. |
| `observed_at` | fecha y hora UTC (ISO 8601) | Momento de la observación. Sirve para saber qué tan nuevo es el dato. |
| `conditions.temperature_c` | número | Temperatura a 2 m, en °C. |
| `conditions.wind_speed_kmh` | número | Viento sostenido a 10 m, en km/h. |
| `conditions.wind_gust_kmh` | número | Ráfagas a 10 m, en km/h. |
| `conditions.wind_direction_deg` | entero, 0 a 360 | Dirección de la que viene el viento (0 = norte). |
| `conditions.visibility_m` | número | Visibilidad horizontal, en metros. |
| `conditions.precipitation_mm` | número | Precipitación de la última hora, en mm. |
| `conditions.thunderstorm` | booleano | Si hay tormenta eléctrica. |
| `operability.operable` | booleano | Si se puede operar, tanto para aterrizar como para despegar. |
| `operability.reasons` | lista de textos | Por qué no se puede operar. Vacía si `operable` es `true`. |

**Cuándo un aeropuerto no es apto**

Alcanza con que se cumpla una de estas condiciones:

| Condición | Umbral | Motivo en `reasons` |
|---|---|---|
| Viento sostenido | más de 50 km/h | `HIGH_WIND` |
| Ráfagas | más de 65 km/h | `HIGH_GUSTS` |
| Visibilidad | menos de 800 m | `LOW_VISIBILITY` |
| Tormenta eléctrica | presente | `THUNDERSTORM` |

Ejemplo de respuesta no apta:

```json
"operability": {
  "operable": false,
  "reasons": ["HIGH_WIND", "HIGH_GUSTS"]
}
```

**Origen de los datos.** Son observaciones reales de [Open-Meteo](https://open-meteo.com/). Las condiciones que forzamos internamente para nuestras pruebas **nunca** salen por esta API.

## Errores

Todos los errores usan el formato Problem Details ([RFC 9457](https://www.rfc-editor.org/rfc/rfc9457)). Para decidir qué hacer, usá el campo `code`: es estable dentro de una misma versión.

```json
{
  "type": "/problems/weather-source-unavailable",
  "title": "Fuente de clima no disponible",
  "status": 503,
  "detail": "La fuente de datos meteorológicos no respondió a tiempo.",
  "instance": "/v1/airports/SACO/conditions",
  "code": "WEATHER_SOURCE_UNAVAILABLE"
}
```

| HTTP | `code` | Qué pasó | ¿Reintentar? |
|---|---|---|---|
| 400 | `INVALID_AIRPORT_CODE` | El código no tiene formato ICAO (por ejemplo `cor` o `SAC`). | **No.** Corregí el pedido. |
| 401 | `MISSING_API_KEY` | Falta el header `X-API-Key`. | **No.** Agregá la clave. |
| 401 | `INVALID_API_KEY` | La clave no corresponde a ningún consumidor. | **No.** Revisá la clave. |
| 404 | `AIRPORT_NOT_SUPPORTED` | El código es válido pero no es un aeropuerto atendido. | **No.** |
| 429 | `RATE_LIMIT_EXCEEDED` | Pasaste los 60 pedidos por minuto. | **Sí**, después de los segundos que indica `Retry-After`. |
| 503 | `WEATHER_SOURCE_UNAVAILABLE` | No pudimos obtener el clima de la fuente. | **Sí**, después de los segundos que indica `Retry-After`. |

Cualquier otro `5xx` se puede reintentar con espera creciente.

## Condiciones de uso

- **Clave.** Cada grupo consumidor recibe su propia clave. No la subas a tu repositorio: guardala como secreto o variable de entorno.
- **Límite.** 60 pedidos por minuto por clave. Cada respuesta `200` trae el header `X-RateLimit-Remaining` con los pedidos que te quedan en el minuto.
- **Idempotencia.** La operación es de solo lectura: repetir un pedido no cambia nada, así que se puede reintentar sin riesgo.
- **Frescura.** El clima cambia con el tiempo; usá `observed_at` para decidir si el dato te sirve.

## Versionado y compatibilidad

- La versión mayor va en la ruta: `/v1/...`.
- **Cambios compatibles**, que no cambian la versión: agregar campos a la respuesta, agregar valores nuevos a `reasons` o a `code`, agregar operaciones o aeropuertos atendidos. Tu cliente tiene que **ignorar los campos que no conoce** y tratar un motivo o código desconocido como genérico.
- **Cambios que rompen**, que crean `/v2`: quitar o renombrar campos, cambiar tipos o unidades, cambiar el significado de un campo o de un código de error. Cuando exista `/v2`, `/v1` se sigue manteniendo.
- Los umbrales de aptitud pueden ajustarse sin cambiar la versión; si cambian, se actualiza este documento.

## Probar con el mock

El mock responde a partir del contrato, con los ejemplos que trae. Acepta cualquier valor en `X-API-Key`, pero exige que el header esté.

**Con Docker**, desde `docs/contracts`:

```sh
docker build -t clima-mock .
docker run --rm -p 4010:4010 clima-mock
```

**Con Node**, desde la raíz del repo:

```sh
npx @stoplight/prism-cli mock docs/contracts/clima-v1.yaml
```

Pedidos de ejemplo:

```sh
# Aeropuerto apto
curl -H "X-API-Key: demo" http://localhost:4010/v1/airports/SACO/conditions

# Aeropuerto no apto
curl -H "X-API-Key: demo" -H "Prefer: example=not_operable" http://localhost:4010/v1/airports/SACO/conditions

# Sin clave → 401
curl http://localhost:4010/v1/airports/SACO/conditions

# Código inválido → 400
curl -H "X-API-Key: demo" http://localhost:4010/v1/airports/cor/conditions
```

El mock no sabe qué aeropuertos atendemos: con cualquier código de cuatro letras responde `200`. Para probar el resto de los errores, pedilos con el header `Prefer`:

```sh
curl -i -H "X-API-Key: demo" -H "Prefer: code=404" http://localhost:4010/v1/airports/SAWH/conditions
curl -i -H "X-API-Key: demo" -H "Prefer: code=429" http://localhost:4010/v1/airports/SACO/conditions
curl -i -H "X-API-Key: demo" -H "Prefer: code=503" http://localhost:4010/v1/airports/SACO/conditions
```

## Historial

| Versión | Fecha | Cambios |
|---|---|---|
| 1.0.0 | 9/10/2026 | Primera versión del contrato, con mock. |
