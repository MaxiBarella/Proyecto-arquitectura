# Decisiones del equipo

Registro de lo que el equipo decidió sobre el sistema. Es la fuente de la que salen los ADR, la documentación de alcance y la de arquitectura.

- **Cuándo:** secciones 1 a 7, reunión del 7/10/2026, cerrada la madrugada del 8/10. Secciones 8 a 20, para la Entrega 2: documento completado por el equipo el 15/10/2026 y cerrado el 16/10.
- **Cómo:** el equipo completó cada documento de decisiones (un Word con opciones por cada punto) y este archivo transcribe lo que eligieron. Lo que eligió una persona o el agente de IA por su cuenta está marcado en cada sección.
- **Numeración:** es la de los documentos de decisiones.

Lo que no figura acá no está decidido. Las opciones descartadas y su análisis van en el ADR correspondiente cuando se escriba.

## El sistema, en un párrafo

Un sistema para administrar los arribos y despegues de un aeropuerto. Las aerolíneas publican sus vuelos y piden un slot de pista para cada uno; el operador del aeropuerto administra las pistas; cualquiera puede ver el tablero de arribos y partidas. El clima puede cerrar una pista, y eso demora los despegues y desvía los arribos. El servicio de clima es la capacidad que se publica para otro grupo.

## 1. Quién usa el sistema

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 1 | Usuarios | **B** | Dos roles con sesión: **aerolínea** (publica arribos y despegues y pide slots) y **operador** del aeropuerto (administra las pistas). El tablero es público; el pasajero mira sin cuenta. |

## 2. Reglas del negocio

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 2.1 | Aeropuertos y pistas | **B** | Un solo aeropuerto. Las pistas se cargan como datos; se arranca con dos. |
| 2.2 | Qué es un slot | **A** | El día de cada pista se divide en bloques fijos e iguales. Un slot es un bloque y sirve igual para aterrizar que para despegar. |
| 2.3 | Cómo se asigna | **A** | La aerolínea pide un slot exacto (pista y horario). Si está libre se lo queda; si no, se rechaza y se le muestran los libres más cercanos. |
| 2.4 | Quién cierra una pista | **C** | Se cierra sola cuando Clima informa que no se puede operar, y el operador también puede cerrarla a mano. |
| 2.5 | Vuelos de una pista cerrada | **D** | Los **despegues** quedan demorados y la aerolínea pide un slot nuevo. Los **arribos** pasan a "desviado". |
| 2.6 | Estados de un vuelo | **B** | Pendiente de slot → Programado → (Demorado → Programado) → Finalizado. Cancelado se alcanza desde cualquiera salvo Finalizado. Desviado es un estado final que solo vale para arribos. |
| 2.7 | Arribos y despegues | **B** | **Los arribos tienen prioridad.** Si un arribo pide un slot ocupado por un despegue, se lo queda, y el despegue pasa a "demorado". Entre dos arribos, o entre dos despegues, gana el primero. |
| 2.8 | Nuestro aeropuerto | **Córdoba** | El sistema maneja el aeropuerto de Córdoba (`SACO`). |
| 2.9 | Destino de un arribo desviado | **El alternativo más cercano disponible** | Hay una lista de aeropuertos cercanos ordenada por distancia. El arribo desviado va al primero que esté disponible. El operador puede marcar un aeropuerto de la lista como "no disponible" para que se saltee; el sistema no consulta el estado de los otros aeropuertos. |

Las decisiones 2.8 y 2.9 las tomó Male el 8/10/2026, al preparar el contrato de Clima. Quedaron confirmadas al mergearse el pull request #7, y están desarrolladas en el [SPEC.md](../SPEC.md).

## 3. Servicios

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 3.1 | Responsabilidades | **Confirmada** | La tabla de abajo. Ningún servicio lee ni escribe la base de otro. |
| 3.2 | Búsqueda del tablero | **A** | Vive dentro de Vuelos, que mantiene el índice de búsqueda a partir de eventos. |
| 3.3 | Usuarios e inicio de sesión | **A** | Un servicio propio, **Usuarios**, guarda los usuarios y entrega los tokens. El gateway los valida. |
| 3.4 | Vuelos seguidos y avisos | No aplica | No hay pasajero con cuenta (decisión 1). |

| Servicio | Qué hace | De qué es dueño | Qué guarda de los otros |
|---|---|---|---|
| **Vuelos** | Publicar arribos y despegues, seguir su estado y buscarlos en el tablero. | Datos del vuelo: tipo de operación, aerolínea, origen, destino, horario, estado. | El identificador del slot asignado. |
| **Pistas** | Asignar slots de aterrizaje y de despegue sin que se pisen. Cerrar y reabrir pistas. | Pistas, bloques de tiempo y qué vuelo ocupa cada uno. | El identificador del vuelo, su tipo de operación y la última aptitud informada por Clima. |
| **Clima** | Informar las condiciones y si se puede operar. Es lo que se publica. | Observaciones de clima y umbrales de aptitud. | Nada. |
| **Usuarios** | Guardar los usuarios y entregar los tokens. | Usuarios y credenciales. | Nada. |

Con Usuarios, los microservicios pasan de tres a **cuatro**, más el API gateway.

## 4. Comunicación

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 4.1 | Publicar vuelo y asignar pista | **A** | Dos pasos. El vuelo nace "pendiente de slot"; Vuelos le pide el slot a Pistas con una llamada síncrona; Pistas asigna y publica "slot asignado"; Vuelos lo consume y pasa el vuelo a "programado". |
| 4.2 | Cómo se entera Pistas del clima | **B** | Clima publica un evento cuando cambia la aptitud. Pistas guarda el último valor y lo usa; no consulta a Clima al asignar. |
| 4.3 | Eventos | Lista del documento | Incluye "slot cedido a un arribo", por la decisión 2.7. Ver tabla. |
| 4.4 | Publicar sin perder eventos | **B** | Transactional Outbox con relay por sondeo. |
| 4.5 | Valores iniciales | Los del ADR-005 | Timeout del pedido de slot: 2 segundos. Reintentos del pedido: hasta 2, con espera creciente desde 200 ms, solo ante fallas transitorias y solo desde Vuelos. Sondeo del relay: cada 1 segundo. Reintentos de un consumidor: hasta 3; después, a la cola de mensajes fallidos. Atraso aceptado del aviso de clima en Pistas: hasta 30 segundos. Son el punto de partida; se ajustan con las pruebas de carga. |
| 4.6 | Idempotencia del pedido de slot | **Por restricción única** | Un vuelo tiene a lo sumo un slot, y lo garantiza la base de Pistas. Repetir el pedido para el mismo vuelo y el mismo slot devuelve el resultado ya guardado. No se envía una clave de idempotencia. |
| 4.7 | Dónde se aplica el Outbox | **En los tres servicios que publican** | Pistas y Vuelos guardan el evento en una tabla de outbox, en la misma transacción de MySQL. Clima guarda el evento pendiente dentro del documento de aptitud, en la misma escritura. |
| 4.8 | Mensajes | **JSON, un exchange topic** | Cada mensaje lleva identificador único, tipo, versión, fecha, identificador de correlación, datos y un número de versión del dato. Se publican en un único exchange de tipo topic; cada consumidor tiene su cola durable y su cola de mensajes fallidos. |
| 4.9 | Consumidores | **Idempotentes** | Confirman el mensaje después de guardar su efecto, y guardan el identificador del mensaje en la misma transacción para no aplicarlo dos veces. |
| 4.10 | Nombre del exchange y campos del mensaje | **`aeropuerto.eventos`; `id`, `type`, `version`, `occurred_at`, `correlation_id`, `data_version` y `data`** | Detalla la 4.8. El exchange es de tipo topic y durable. `type` es la clave de ruteo, `version` es la versión del formato del evento (empieza en 1) y `data_version` es el número de versión del dato. Lo propuso Male el 19/10/2026 al programar Clima; queda confirmada al mergearse el pull request #20. |

Las decisiones 4.5 a 4.9 las tomó Salvador el 8/10/2026, al escribir el [ADR-005](adr/ADR-005.md). Quedaron confirmadas al mergearse el pull request #8.

Las decisiones 4.11 a 4.13 las tomó Maxi el 21/10/2026, al cerrar el contrato entre Vuelos y Pistas: Carola, que programa Pistas, le delegó la elección. Quedan confirmadas al mergearse el pull request #18.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 4.11 | Cómo se identifican la pista y el bloque | **Pista: un nombre sin barra, como `05-23`. Bloque: su hora de inicio en UTC** | El nombre lo pone Pistas al cargar las dos pistas (RN-02). Va sin barra para que no rompa rutas como `/runways/05-23/close`. El bloque se indica con la hora en que empieza, en múltiplos de 15 minutos (RN-03), por ejemplo `2026-10-23T14:30:00Z`. |
| 4.12 | Datos de los eventos entre Vuelos y Pistas | **Lo mínimo que usa el que los recibe** | Ver la tabla de abajo. Pistas no guarda los datos del vuelo, que son de Vuelos: solo su identificador y su tipo, que Vuelos manda al pedir el slot porque Pistas lo necesita para la prioridad de los arribos (RN-11). |
| 4.13 | Cómo se asegura que las colas existan antes del primer evento | **Un archivo de definiciones de RabbitMQ** | Al arrancar, antes que los servicios, RabbitMQ crea el exchange, la cola de cada consumidor, sus uniones y sus colas de mensajes fallidos (4.8). Así no se pierde un evento publicado antes de que arranque el que lo recibe. Además, un publicador no saca un evento del outbox si RabbitMQ se lo devuelve por no tener cola. El archivo se suma al `docker-compose.yml`, que edita la pareja 2. |

| Evento | Datos (4.12) |
|---|---|
| Slot asignado | `flight_id`, `runway_id` y `block_start`. |
| Slot cedido a un arribo | `flight_id` del despegue que lo pierde, `runway_id`, `block_start` y `taken_by_flight_id`, el arribo que se lo queda. |
| Slot liberado por cierre | `flight_id`, `runway_id`, `block_start` y `diverted_to`: el código ICAO del alternativo si es un arribo (RN-25), o `null` si es un despegue. |
| Pista cerrada / reabierta | `runway_id` y `reason`: `weather` si la cerró el clima, `operator` si la cerró el operador. |
| Vuelo cancelado | `flight_id`. |

| Evento | Quién lo publica | Quién lo recibe y para qué |
|---|---|---|
| Slot asignado | Pistas | Vuelos: pasa el vuelo a "programado" y actualiza el tablero. |
| Slot liberado por cierre | Pistas | Vuelos: pasa el despegue a "demorado" y el arribo a "desviado". Actualiza el tablero. |
| Slot cedido a un arribo | Pistas | Vuelos: pasa a "demorado" el despegue que perdió su slot. |
| Pista cerrada / reabierta | Pistas | Vuelos: para mostrarlo en el tablero. |
| Aptitud operativa cambió | Clima | Pistas: cierra o reabre las pistas. |
| Vuelo cancelado | Vuelos | Pistas: libera el slot. |
| Clima sin datos | Clima | Pistas: deja de cerrar y reabrir solo y muestra el aviso al operador (6.19). Se sumó en la Entrega 2 (sección 18, pendiente 17). |

La 4.14 la tomó Maxi el 21/10/2026 y queda confirmada al mergearse el pull request #26; la 4.15 y la 4.16, al mergearse el pull request #27. Las claves y la ruta de Pistas las eligió por Carola, que le delegó la elección.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 4.14 | Claves de ruteo de los eventos | **Servicio, cosa y qué pasó, en español, como las de Clima (9.14)** | `vuelos.vuelo.cancelado`; `pistas.slot.asignado`, `pistas.slot.cedido`, `pistas.slot.liberado_por_cierre`, `pistas.pista.cerrada` y `pistas.pista.reabierta`. Así un consumidor puede pedir todo lo de un servicio con `pistas.#`. |
| 4.15 | Cómo pide Vuelos el slot a Pistas | **`POST /internal/slots` en Pistas** | Con `flight_id`, `flight_type`, `runway_id` y `block_start`. Responde `200` con el slot asignado (también si se repite el mismo pedido, 4.6), o `409` con `SLOT_TAKEN` y los 3 libres, `RUNWAY_CLOSED` o `FLIGHT_ALREADY_HAS_SLOT`, o `404` con `RUNWAY_NOT_FOUND`, o `503` con `UNAVAILABLE` ante una falla pasajera, como que no responda su base, y Vuelos lo reintenta (4.5; se sumó en el pull request #40). Es interna: el gateway no la expone (15.7). El contrato está en `services/pistas/openapi.yaml`. La eligió Maxi por Carola, que le delegó la elección. |
| 4.16 | Los 2 segundos del pedido de slot (4.5) | **Son por intento** | Cada llamada a Pistas tiene 2 segundos. Si Pistas no contesta nunca, la aerolínea espera unos 7 segundos: 3 intentos más las esperas. Así los reintentos sirven también cuando Pistas está lento. |

Las decisiones 4.17 a 4.20 detallan la 4.13. Las tomó Male el 22/10/2026, con las opciones y recomendaciones del agente, y quedan confirmadas al mergearse el pull request de la rama `feat/rabbitmq-definiciones`. El archivo y su explicación están en [rabbitmq/](../rabbitmq/README.md).

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 4.17 | Cómo se cargan las definiciones de RabbitMQ | **Un contenedor de un solo uso, `rabbitmq-init`** | RabbitMQ arranca con el usuario del `.env`; cuando está listo, `rabbitmq-init` sube `rabbitmq/definitions.json` por la API de administración y termina, y recién ahí arrancan los servicios. Se descartó cargarlo al arrancar RabbitMQ (`load_definitions`): se probó, y así RabbitMQ no crea el usuario del `.env`; ponerlo en el archivo dejaría la contraseña en el repositorio. |
| 4.18 | Nombres de las colas | **`<consumidor>.eventos` y `<consumidor>.eventos.fallidos`** | `pistas.eventos`, `pistas.eventos.fallidos`, `vuelos.eventos` y `vuelos.eventos.fallidos`. |
| 4.19 | Qué recibe cada cola | **Lo que sale de la 4.3, la 4.14 y la 9.14** | `pistas.eventos` recibe `clima.#` y `vuelos.vuelo.cancelado`; `vuelos.eventos` recibe `pistas.#`. |
| 4.20 | Cómo llega un mensaje a la cola de fallidos | **Por el exchange `aeropuerto.fallidos` de RabbitMQ** | Cada cola tiene configurado ese exchange para los mensajes rechazados. El consumidor, después de 3 reintentos (4 intentos en total, 4.5 y 4.24), rechaza el mensaje sin volver a encolarlo, y RabbitMQ lo pasa solo a su cola de fallidos (11.4). |

Las decisiones 4.21 a 4.24 las tomó Maxi el 22/10/2026, al programar el relay y el consumidor de Vuelos. Quedan confirmadas al mergearse el pull request #29.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 4.21 | Relay con dos instancias de Vuelos | **Cada una toma eventos que la otra no tiene** | El relay lee el outbox con `SELECT ... FOR UPDATE SKIP LOCKED`: si una instancia está publicando unos eventos, la otra los saltea. Ninguno sale dos veces y, si una instancia se cae, la otra sigue sola. |
| 4.22 | Evento que no se puede aplicar | **Se confirma y queda un aviso en el log** | Por ejemplo, "slot asignado" para un vuelo que la aerolínea ya canceló, o un tipo de evento que Vuelos no conoce. Reintentar no lo arregla, y mandarlo a fallidos dispararía la alerta de la 11.4 sin un error real. Pistas igual libera el slot al recibir "vuelo cancelado". |
| 4.23 | Orden de los eventos que recibe Vuelos | **Un solo consumidor activo en `vuelos.eventos`** | La cola tiene `x-single-active-consumer`: RabbitMQ la entrega a una instancia por vez y la otra queda de reserva. Así los eventos de un mismo vuelo se aplican en orden; por ejemplo, "slot asignado" antes que "slot cedido". |
| 4.24 | Esperas entre reintentos del consumidor (4.5) | **1, 2 y 4 segundos** | Ante una falla pasajera, como MySQL sin responder. Después del tercer reintento el mensaje va a fallidos (4.20). |

## 5. Datos

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 5.1 | Tipo de base por servicio | **A** | Vuelos y Pistas, relacional. Clima, no relacional. |
| 5.2 | Cuántas bases | **A** | Instancia separada: cada servicio levanta su propio servidor de base de datos. |
| 5.3 | Base del servicio Usuarios | **MySQL, instancia propia** | Con una restricción única sobre el nombre de usuario. Las contraseñas se guardan hasheadas. |
| 5.4 | Acceso a las bases | **GORM y driver oficial** | Vuelos, Pistas y Usuarios acceden a MySQL con GORM. Clima usa el driver oficial de MongoDB para Go. |
| 5.5 | Estructuras iniciales | Las del ADR-003 | Pistas: pistas, slots con restricción única sobre pista y bloque, outbox y mensajes procesados. Vuelos: vuelos, outbox y mensajes procesados. Clima: observaciones y un documento de aptitud por aeropuerto. Usuarios: usuarios. |

Las decisiones 5.3 a 5.5 las tomó Salvador el 8/10/2026, al escribir el [ADR-003](adr/ADR-003.md). Quedaron confirmadas al mergearse el pull request #8.

Las decisiones 5.6 a 5.9 las tomó Maxi el 21/10/2026, al programar cómo guarda Vuelos sus datos. Quedan confirmadas al mergearse el pull request #24. Valen para Vuelos; Pistas y Usuarios pueden seguir el mismo modelo.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 5.6 | Cómo se crean las tablas | **Un script SQL que MySQL corre al crear la base** | Está en `services/vuelos/db/`. Por qué: las dos instancias de Vuelos (12.1) arrancan a la vez y así ninguna compite por crear las tablas; además los tests de integración usan el mismo archivo. Lo que se paga: si una tabla cambia, hay que borrar el volumen con `docker compose down -v`. |
| 5.7 | Dos cambios a la vez sobre el mismo vuelo | **Bloquear la fila mientras se cambia** | El vuelo se lee con `SELECT ... FOR UPDATE` dentro de la transacción: el segundo cambio espera al primero y trabaja sobre el estado nuevo. Por ejemplo, una cancelación y un "slot asignado" que llegan juntos. |
| 5.8 | Cómo se generan los identificadores | **Una función propia, como en Clima** | Un UUID versión 4 hecho con `crypto/rand`, sin sumar librerías. |
| 5.9 | Cómo se conecta Vuelos a MySQL | **Usuario propio y la variable `MYSQL_DSN`** | La imagen de MySQL crea un usuario con acceso solo a la base `vuelos`. El Docker Compose arma `MYSQL_DSN` con ese usuario. Vuelos no usa root. |

## 6. Clima hacia afuera

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 6.1 | Qué ofrece | **B** | Condiciones actuales por aeropuerto y aptitud operativa según umbrales propios. Sin avisos por evento hacia afuera. |
| 6.2 | Origen de los datos | **C** | Datos reales, con la posibilidad de que el operador fuerce una condición. |
| 6.3 | Formato del contrato | **A** | API descrita con OpenAPI. |
| 6.4 | Quién puede usarla | **B** | Una clave por consumidor, enviada en cada pedido. |
| 6.5 | Versionado | **A** (reemplazada por la 9.10) | La versión va en la dirección (`/v1/...`). Los cambios compatibles no cambian la versión; los que rompen crean una nueva y la anterior se mantiene. |
| 6.6 | Mock | **A** | Generado a partir del archivo del contrato. |

Las decisiones 6.7 a 6.19 surgieron al preparar el contrato de Clima. Las tomó Male el 8/10/2026 y quedaron confirmadas al mergearse el pull request #7.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 6.7 | Forma de la aptitud | **Una por aeropuerto** | Clima informa si se puede operar o no, sin separar aterrizaje de despegue ni distinguir pistas. Si no se puede, se cierran todas las pistas para las dos cosas (2.2, 2.4 y 2.5). |
| 6.8 | Fuente real | **Open-Meteo** | La del práctico de la clase 7. Trabaja con coordenadas, así que Clima guarda la latitud y longitud de nuestro aeropuerto. Forzar una condición (6.2) sirve para probar el cierre de pistas. |
| 6.9 | Aeropuertos atendidos | **Solo el nuestro** | La API atiende únicamente Córdoba (`SACO`, decisión 2.8). Los aeropuertos alternativos de la 2.9 no pasan por Clima. |
| 6.10 | Identificador del aeropuerto | **Código ICAO** | Por ejemplo `SACO`. |
| 6.11 | Umbrales de aptitud | **Simplificados** | No se opera con viento sostenido de más de 50 km/h, ráfagas de más de 65 km/h, visibilidad de menos de 800 m o tormenta eléctrica. |
| 6.12 | Rutas | **Un endpoint** | `GET /airports/{icao}/conditions` (sin `/v1` desde la 9.10) devuelve las condiciones y la aptitud juntas. Forzar una condición es una ruta interna que no se publica. |
| 6.13 | Envío de la clave | **Header `X-API-Key`** | La clave no va en la URL para que no quede en los logs. |
| 6.14 | Formato de los errores | **Problem Details (RFC 9457)** | Con un campo `code` propio. No se reintentan 400, 401 y 404; se reintentan 429 y 503, con `Retry-After`. |
| 6.15 | Límite de pedidos | **60 por minuto por clave** | Al pasarlo se responde 429 con `Retry-After`. Protege a Clima, y a Open-Meteo detrás, de las pruebas de carga de otros grupos. El valor se puede ajustar. |
| 6.16 | Campos y unidades | **Inglés y métricas** | La unidad va en el nombre del campo (`wind_speed_kmh`). Toda respuesta incluye `observed_at`. |
| 6.17 | Herramienta y publicación del mock | **Prism, local** | El mock se levanta desde el archivo del contrato y corre local, con un `Dockerfile` listo para publicarlo. Si se publica o no se decide después. |
| 6.18 | Alcance del clima forzado | **Solo nuestro sistema** | Forzar una condición afecta únicamente a Pistas. La API pública devuelve siempre el dato real de Open-Meteo, para no alterar las pruebas del grupo consumidor. |
| 6.19 | Si Open-Meteo no responde | **Aviso y operación manual** | Hacia afuera, la API responde `503` con `Retry-After`. Adentro, las pistas no se cierran ni se reabren solas: se muestra un aviso en pantalla y el operador decide a mano (2.4). |

## 7. Tecnologías

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 7.1 | Un lenguaje o varios | **A** | El mismo para todos los servicios: Go con Gin. |
| 7.2 | Las piezas | Las sugeridas en la tabla | Ver abajo. |
| 7.3 | Patrones internos | **B** | Pistas, Hexagonal. Vuelos, CQRS. Clima, en capas. |
| 7.4 | API gateway | **NGINX** | El mismo NGINX del balanceo, con un archivo de configuración por rol. Se decidió en la Entrega 2, después de la respuesta del profe (sección 15). |
| 7.5 | Patrón interno de Usuarios | **Capas** | Igual que Clima: controllers, services, repositories y models. |
| 7.6 | Estructura de carpetas | `services/<nombre>` | Cada servicio tiene `cmd/api` como punto de entrada y su código en `internal/`, ordenado según su patrón. Un módulo de Go por servicio. |
| 7.7 | Arranque local | **Un `docker-compose.yml` en la raíz** | Puertos locales: Vuelos 8081, Pistas 8082, Clima 8083 y Usuarios 8084. Imágenes: MySQL 8.4, MongoDB 7, RabbitMQ 3.13 y Solr 9. Las contraseñas salen de un `.env` que no se sube. |

Las decisiones 7.5 a 7.7 las tomó Salvador el 8/10/2026, al armar la estructura inicial. Quedaron confirmadas al mergearse el pull request #9.

| Pieza | Elegido |
|---|---|
| Lenguaje y framework | Go con Gin |
| Frontend | React |
| Base relacional | MySQL |
| Base no relacional | MongoDB |
| Motor de búsqueda | Solr |
| Mensajería | RabbitMQ |
| Balanceador de carga | NGINX |
| API gateway | NGINX, el mismo del balanceo (15.1) |
| Arranque local | Docker Compose |
| Caché | Memcached (10.2) |
| Observabilidad | OpenTelemetry en los servicios y Grafana en un contenedor (11.1) |
| Hosting de Clima | Render, con MongoDB Atlas (9.4, 9.7) |
| Pruebas de carga | k6 (16.1) |

## Decisiones de la Entrega 2

Las secciones 8 a 20 salen del documento de decisiones de la Entrega 2 (del 12/10/2026), que el equipo completó el 15/10/2026.

Quién eligió cada cosa:

- Lo que no tiene marca lo eligió el equipo en el documento.
- La 15.1, la 15.3 y la 15.4 las eligió Maxi el 16/10/2026, con la respuesta del profe.
- Lo marcado **(agente)** lo eligió el agente de IA el 16/10/2026, a pedido de Maxi, con el criterio de lo que más simplifica el trabajo. Son puntos que el documento dejó sin completar o que aparecieron después. Quedan confirmados cuando el equipo apruebe el pull request que los registra; si alguien no está de acuerdo, se discute ahí.

## 8. Alcance del código

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 8.1 | Qué se construye para el 23/10 | **B** | Clima real, completo y publicado. Vuelos y Pistas con el camino principal (publicar un vuelo, pedir un slot libre y recibir "slot asignado"), más los pedidos simultáneos y la prioridad de los arribos. El cierre de pistas, las demoras por clima y los desvíos quedan para noviembre. Por qué: para adelantar trabajo. |

## 9. Clima real

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 9.1 | Cómo obtiene los datos | **A. Consulta periódica** | Clima le pregunta a Open-Meteo cada cierto tiempo, guarda la observación en MongoDB y calcula la aptitud. Las consultas de afuera leen el último dato guardado. Por qué: la aptitud está disponible sin que nadie tenga que consultar. |
| 9.2 | Intervalo y antigüedad máxima | **Cada 15 minutos; viejo a los 30** | Open-Meteo actualiza cada 15 minutos, más o menos: consultar más seguido trae lo mismo. Si el último dato tiene más de 30 minutos, Clima responde `503` hacia afuera y avisa que no tiene datos (6.19). |
| 9.3 | El Clima de la nube y el nuestro | **A. El mismo código, dos copias** | Una copia en la nube atiende a los otros grupos y no usa RabbitMQ. Otra corre en el Docker Compose y le avisa a Pistas. Las dos leen la misma fuente. Por qué: no subir a la nube más de lo necesario. |
| 9.4 | Dónde se publica | **A. Render** | Despliega el contenedor desde el repo. En el plan gratuito el servicio se duerme sin uso y el primer pedido tarda: lo resuelve el consumidor con su timeout y sus reintentos. Por qué: se le preguntó al profe y no recomendó otra cosa. |
| 9.5 | Claves de los consumidores | **A. Variable de entorno** | Se cargan como configuración secreta del hosting, sin subirlas al repo ni guardarlas en la base. |
| 9.6 | Dónde se aplica el límite de 60 pedidos por minuto (6.15) | **En Clima** (agente) | La copia de la nube no tiene gateway adelante (9.3), así que el límite tiene que estar en el servicio. Se cuenta en memoria, por clave: alcanza porque en la nube corre una sola instancia. |
| 9.7 | MongoDB de la copia de la nube | **MongoDB Atlas, plan gratuito** (agente) | Render no ofrece MongoDB y Atlas es la opción gratuita habitual. Antes de crearlo hay que verificar las condiciones del plan. |
| 9.8 | Publicación del mock (6.17) | **No se publica** (agente) | Se publica directamente el Clima real. El mock queda en el repo para pruebas locales. |
| 9.9 | Por dónde entra el grupo consumidor | **Directo a Clima en la nube, sin gateway** (agente) | Lo había propuesto Carola en `docs/ARCHITECTURE.md` y es lo que resulta de la 9.3. Conviene confirmarlo con el profe (ver preguntas). |
| 9.10 | Versionado del contrato de Clima | **Sin versión en la ruta; con el número del contrato** | Por indicación del profe, la ruta queda `GET /airports/{icao}/conditions`. La versión es el número del contrato (`info.version`, con el formato mayor.menor.parche) y el historial de la guía. Un cambio que rompe sube el número mayor y se avisa en la guía. Reemplaza a la 6.5. Lo decidió el equipo en una llamada. |
| 9.11 | Timeout recomendado al consumidor | **Al menos 60 segundos en el primer pedido** | Por el servicio dormido de Render (9.4), el primer pedido después de un rato sin uso tarda cerca de un minuto. Se recomienda en la guía del contrato. Lo decidió el equipo en una llamada. |

Las decisiones 9.12 a 9.18 surgieron al programar Clima. Las tomó Male el 19/10/2026, con las opciones y recomendaciones del agente; la 9.14 la revisa la pareja 1 en el pull request, porque Pistas consume esos eventos.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 9.12 | Qué se fuerza | **Valores de clima** | El operador fuerza viento, ráfagas, visibilidad o tormenta, y la aptitud se calcula con los umbrales de la 6.11. Así se prueban los umbrales de punta a punta. |
| 9.13 | Ruta interna del forzado | **`PUT` y `DELETE` en `/internal/airports/SACO/forced-conditions`** | `PUT` pone la condición forzada y `DELETE` la quita. Solo la acepta con `X-User-Role: operator` (15.6). La copia de la nube no la expone. |
| 9.14 | Eventos de Clima | **`clima.aptitud.cambio` y `clima.sin_datos`** | Son las claves de ruteo. "Aptitud cambió" lleva aeropuerto, `operable`, motivos, fecha de la observación y si es forzada; se publica solo cuando `operable` cambia, y también cuando vuelven los datos después de un "sin datos". "Sin datos" se publica cuando el último dato pasa los 30 minutos (9.2), y lleva solo el aeropuerto y la fecha del último dato. En "aptitud cambió", `reasons` va siempre, como lista, vacía si se puede operar. Si ninguna cola recibe un evento, queda en el outbox hasta que exista una: no se pierde. |
| 9.15 | Variables de entorno de Clima | **`CLIMA_API_KEYS`, `MONGO_URI`, `RABBITMQ_URL` y `RABBITMQ_ENABLED`** | Las claves van como `grupo1:clave1,grupo2:clave2`. `RABBITMQ_ENABLED=false` apaga la publicación de eventos en la copia de la nube (9.3). |
| 9.16 | Historial de observaciones | **Se borran solas a los 7 días** | Con un índice TTL de MongoDB. El servicio solo usa la última, y así el plan gratuito de Atlas no se llena. |
| 9.17 | Cómo se cuenta el límite de 60 por minuto | **Un balde de 60 que se rellena de a 1 por segundo** | Permite ráfagas de hasta 60 y en promedio son 60 por minuto; en el primer minuto, con el balde lleno, pueden pasar hasta 119. `Retry-After` dice los segundos hasta que haya lugar; en el `503` dice 60. |
| 9.18 | Precipitación | **La de los últimos 15 minutos** | Es lo que informa Open-Meteo. Se corrige el texto del contrato, que decía "última hora": pasa a 2.0.1, una aclaración. |
| 9.19 | Si falla una consulta a Open-Meteo | **Se reintenta al minuto** | En vez de esperar los 15 minutos. Como el dato ya puede tener hasta 15 minutos, una sola falla no llega a dejarlo viejo. Surgió en la revisión del pull request #20; lo eligió Male el 20/10/2026. |
| 9.20 | Plazo de cada tarea periódica de Clima | **30 segundos** | La consulta, la revisión de la aptitud y el relay se cortan a los 30 segundos y vuelven a intentar. Así, si MongoDB o RabbitMQ se cuelgan, la tarea no queda trabada. Un mismo error se registra como mucho una vez por minuto. Surgió en la revisión del pull request #20; lo eligió Male el 20/10/2026. |
| 9.21 | Qué revisa `/health` de Clima | **Solo que el servicio esté vivo; no mira MongoDB** | Clima no está detrás del balanceador (el que se balancea es Vuelos, 12.1). Si mirara MongoDB, una caída de Atlas haría que Render reiniciara Clima una y otra vez, cuando Clima igual puede responder `503` con un error claro. Surgió en la revisión del pull request #20; lo eligió Male el 20/10/2026. |

## 10. Caché (ADR D7)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 10.1 | Qué se cachea | **A. El tablero público** | Las consultas del tablero, en Vuelos. Se mide primero sin caché para tener el número de "antes". Por qué: es lo más leído del sistema. |
| 10.2 | Herramienta | **Memcached** | Una sola, compartida por las dos instancias de Vuelos (12.1). Por qué: se usó en el práctico de la clase 3. |
| 10.3 | Cómo se mantiene al día | **A. Solo vencimiento, 60 segundos** | Cada entrada dura 60 segundos y después se vuelve a calcular. No se borra cuando llega un evento. Por qué: con un vencimiento corto, borrar por evento casi no cambia nada. |
| 10.4 | Si Memcached no responde | **Se consulta Solr directo** (agente) | El tablero sigue funcionando, más lento, y queda un log del error. La caché nunca es imprescindible para responder. |
| 10.5 | Cuánto se espera a Memcached | **100 ms** (Maxi, 9/11/2026; ver la 28) | Pasado ese plazo se va directo a Solr (10.4). En la misma red, Memcached responde en menos de 1 ms: 100 ms da margen sin que el tablero se vuelva lento si se cae. |
| 10.6 | Memoria de Memcached | **128 MB** (Maxi, 9/11/2026; ver la 28) | Cada página del tablero pesa unos 8 KB, así que 64 MB, que es lo que trae, alcanzan para unas 8000 consultas distintas. 128 MB deja margen en la prueba de carga (16.2) y no cuesta nada. Se fija en el `docker-compose.yml`. |
| 10.7 | Cómo se apaga la caché para medir el "antes" | **La variable `CACHE_ENABLED=false` en Vuelos** (Maxi, 9/11/2026; ver la 28) | Se descartó levantar sin el contenedor de Memcached: la medición se ensuciaría con los errores de conexión y el plazo de la 10.5. |
| 10.8 | Qué consultas manda k6 para medir la caché | **80 % la portada y 20 % con filtros al azar** (Maxi, 9/11/2026; ver la 28) | La mezcla que propone el ADR-007, la misma en las dos corridas. Con todas iguales casi todo saldría de la caché; con todas al azar, casi nada. Es el valor por defecto de `PORCENTAJE_PORTADA` en `tests/carga/tablero.js`. |
| 10.9 | Con qué datos se mide la caché | **Un script aparte, `tests/carga/sembrar-tablero.js`, que se corre una vez antes de las dos corridas** (Maxi, 9/11/2026; ver la 28) | Publica 700 vuelos en los próximos 7 días, unos 100 por día, del orden de los que opera Córdoba, de las dos aerolíneas, arribos y despegues, nacionales e internacionales, y a 3 de cada 4 les pide slot. Las dos corridas miden sobre los mismos datos. Se descartó cargarlos dentro de `tablero.js`: la segunda corrida tendría el doble de vuelos. Y se descartó medir con el índice casi vacío: Solr responde rápido igual y el "antes" sale optimista. |
| 10.10 | Qué variedad tiene el 20 % con filtros (10.8) | **Miles de combinaciones: tipo, estado, aerolínea, fecha de los próximos 7 días, búsqueda, orden y páginas 1 a 5, de uno a tres filtros a la vez** (Maxi, 9/11/2026; ver la 28) | Con los filtros anteriores había unas 100 consultas distintas: con 60 segundos de vigencia, a 100 pedidos por segundo ya estaban todas en la caché y el ensayo dio 99,6 % de aciertos. Así el 20 % casi nunca acierta, que es lo que busca la 10.8, y el resultado depende sobre todo de la portada. Se descartó medir con las dos variedades: eran 4 corridas y más números para explicar. |

## 11. Observabilidad (ADR D11)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 11.1 | Dónde se ven logs, métricas y trazas | **A. Grafana en un contenedor** | Los servicios emiten todo con OpenTelemetry hacia una sola imagen que trae Grafana y sus almacenes ya configurados (`grafana/otel-lgtm`). Por qué: menos contenedores y menos peso en el arranque local. |
| 11.2 | Cómo se correlaciona un pedido | **A. El identificador de la traza** | El que genera OpenTelemetry. En HTTP viaja solo; en los eventos de RabbitMQ se copia en el campo de correlación que ya tiene cada mensaje (4.8). Por qué: un solo identificador para logs y trazas. |
| 11.3 | Objetivo de servicio | **Pedido de slot: el 95 % en menos de 500 ms** | Medido sobre los últimos 5 minutos. Es una primera versión: todo corre en una sola computadora y en los picos de la prueba de carga algunos pedidos van a tardar más. |
| 11.4 | Mensajes fallidos (pendiente de la Entrega 1) | **Log de error y alerta en Grafana** (agente) | Cuando un consumidor manda un mensaje a la cola de fallidos, escribe un log de error con el identificador de correlación, y Grafana alerta cuando aparece uno. Se revisan a mano desde el panel de RabbitMQ. Sirve como la alerta que pide el ADR D11. |
| 11.5 | Formato de los logs | **JSON con `log/slog`** (agente) | Es la librería de logs que trae Go. Cada línea lleva el identificador de la traza. |
| 11.6 | Cómo sigue la traza en los eventos | **El encabezado `traceparent` en el mensaje, guardado en el outbox** (Male, 22/10/2026) | Detalla la 11.2. Con solo el `correlation_id`, Grafana muestra trazas separadas. Por eso, al guardar un evento en el outbox se guarda también el `traceparent` de la traza en curso; el relay lo pone como encabezado del mensaje de RabbitMQ, y el consumidor lo lee para seguir la misma traza. Las funciones para escribirlo y leerlo van en la carpeta de observabilidad (11.1), que cada servicio copia: la escribe Male en Clima, y la pareja 1 la usa en los relays y consumidores de Vuelos y Pistas. Queda confirmada al mergearse el pull request de la rama `feat/rabbitmq-definiciones`. |

Las decisiones 11.7 a 11.10 son los pendientes del ADR-011. Las propuso Male al grupo el 7/11/2026, con las opciones del agente, y las confirmó Maxi en la revisión del pull request #46 el 9/11/2026.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 11.7 | Qué muestra el tablero | **Pedidos y errores por servicio, el p95 del pedido de slot contra el objetivo, el reparto entre las instancias de Vuelos y los eventos enviados a fallidos** | Es el tablero "Aeropuerto: estado general", en `grafana/dashboards/`, que Grafana carga al arrancar (20.4). |
| 11.8 | La alerta de fallidos se apaga sola | **Se acepta, como limitación** | La alerta de la 11.4 mira los últimos 5 minutos de logs: si nadie revisa el mensaje, deja de sonar aunque siga en la cola. Alertar por la cantidad de mensajes en las colas `.fallidos` necesita sacar métricas de RabbitMQ, y no entraba antes de la presentación. |
| 11.9 | Alerta por el objetivo de servicio | **Si el p95 del pedido de slot pasa los 500 ms durante 5 minutos seguidos** | Está en `grafana/alerting/objetivo.yaml`. Usa la misma consulta que la 11.3. |
| 11.10 | Si en 5 minutos no hubo pedidos de slot | **El tablero muestra "Sin datos", en gris, y la alerta no se dispara** | Sin pedidos no hay un p95 que medir: mostrar 0 diría que se cumple el objetivo, y disparar la alerta avisaría de un problema que no existe. |

## 12. Balanceo de carga (ADR D12)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 12.1 | Qué servicio se balancea | **A. Vuelos** | Dos instancias de Vuelos detrás de NGINX. Por qué: es el que más tráfico recibe y tiene el tablero cacheado, así que caché y balanceo se muestran sobre el mismo flujo. |
| 12.2 | Cómo reparte | **A. Por turnos** | El comportamiento por defecto de NGINX. Una instancia caída se detecta de forma pasiva, porque la verificación activa es de la versión paga; el ADR explica por qué alcanza. Por qué: es lo más simple. |
| 12.3 | Eventos duplicados con dos instancias | **Se aceptan** (agente) | Las dos instancias revisan la misma tabla de outbox, pero no publican a la vez el mismo evento: cada una toma los que la otra no tiene (4.21). Un evento todavía puede salir dos veces si una instancia se cae entre publicarlo y borrarlo. No rompe nada porque los consumidores son idempotentes (4.9). De `vuelos.eventos` consume una sola instancia por vez (4.23), así que cada evento que le llega a Vuelos lo procesa una sola. |

La 12.3 se actualizó el 23/10/2026 con la 4.21 y la 4.23, que cambiaron cómo se reparten el outbox y la cola entre las dos instancias. La decisión de fondo no cambió: un evento todavía puede salir dos veces, y se acepta.

Las decisiones 12.4 a 12.7 las tomó Salvador el 9/11/2026, al armar las dos instancias de Vuelos (`nginx/balanceo.conf` y `docker-compose.yml`). Eran los pendientes del [ADR-012](adr/ADR-012.md). Las opciones y las recomendaciones las preparó el agente de IA. Quedan confirmadas al mergearse el pull request que las registra, que revisa la pareja 1.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 12.4 | Cuándo NGINX da por caída una instancia | **Con 1 falla, y la vuelve a probar a los 10 segundos** | Es lo que trae NGINX (`max_fails=1`, `fail_timeout=10s`). En la demo se ve enseguida cómo la otra absorbe todo. Si la falla fue pasajera, la otra instancia aguanta 10 segundos sola. Se descartó pedir 3 fallas: más pedidos fallan antes de sacarla. |
| 12.5 | Cuánto espera NGINX a un servicio | **1 segundo para conectar y 10 para responder** | En la red de Docker conectar tarda milisegundos: si no conecta en 1 segundo, la instancia está colgada y NGINX prueba con la otra. 10 segundos cubren el peor caso del pedido de slot, unos 6,6 segundos: 3 intentos de 2 segundos a Pistas, más las esperas (4.5). Vale para todos los servicios, porque NGINX lo configura en el gateway y no por instancia. Se descartaron los 60 segundos que trae NGINX: una instancia colgada haría esperar un minuto. |
| 12.6 | Cómo se declaran las instancias | **Dos servicios, `vuelos-1` y `vuelos-2`** | La configuración se escribe una vez (`x-vuelos` en el `docker-compose.yml`) y cambia solo el nombre. Cada una se puede apagar por separado para el ensayo (`docker compose stop vuelos-2`). El servicio `vuelos` deja de existir: los comandos usan `vuelos-1` o `vuelos-2`. Se descartó un servicio con dos réplicas, que no se pueden nombrar ni apagar de a una. |
| 12.7 | Cómo se distingue cada instancia en Grafana | **`OTEL_RESOURCE_ATTRIBUTES=service.instance.id=vuelos-1` en el compose** | OpenTelemetry lee esa variable solo, así que no se toca el código de Vuelos. En Prometheus aparece como la etiqueta `service_instance_id`. Se descartó copiar el cambio de Clima del #47, que usa el nombre de la máquina: tocaba el código de la pareja 1. |

## 13. Resiliencia (ADR D10)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 13.1 | Dónde va el mecanismo de protección | **B. Circuit breaker de Vuelos a Pistas** | Si Pistas viene fallando, Vuelos deja de llamarla un rato y los vuelos quedan "pendiente de slot". Se suma al timeout y los reintentos de la 4.5. Por qué: el efecto se ve en el frontend. |
| 13.2 | Valores del circuit breaker | **5 fallas seguidas, 30 segundos abierto, 1 pedido de prueba** (Maxi, 21/10/2026) | Cuenta como falla un pedido que no consiguió respuesta de Pistas después de los reintentos, o al que Pistas respondió con un error suyo (por ejemplo, `500`). Un rechazo, como slot ocupado o pista cerrada, no cuenta. Abierto, Vuelos responde `503` enseguida, con `Retry-After: 30`. Pasados los 30 segundos deja pasar un pedido: si anda, se cierra; si falla, se abre otra vez. Queda confirmada al mergearse el pull request #27. |

## 14. Búsqueda (ADR D6)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 14.1 | Filtros y orden del tablero | Ver al lado | Filtros: tipo de operación, estado, aerolínea y fecha (agente: el documento quedó sin marcar y son los recomendados). Búsqueda por texto: número de vuelo y ciudad. Orden por defecto: horario. Otros órdenes: aerolínea y ámbito, nacional o internacional. |
| 14.2 | Atraso máximo del índice | **60 segundos** | Desde que cambia un vuelo hasta que se ve en la búsqueda. Por qué: es una espera razonable para un tablero. |
| 14.3 | Cómo se reconstruye el índice | **A. Comando manual** | Lee los vuelos de MySQL y los vuelve a cargar en Solr. Por qué: si fuera automático al arrancar, las dos instancias de Vuelos lo harían a la vez. |
| 14.4 | Paginación | **20 vuelos por página** (agente) | El enunciado pide paginación sin dar un valor. |
| 14.5 | Cómo se entera el índice de cada cambio | **Un evento de Vuelos por RabbitMQ** | Cada cambio de un vuelo guarda en el outbox, en la misma transacción, el evento `vuelos.vuelo.actualizado`, con los datos que muestra el tablero. El relay lo publica y Vuelos lo lee de la cola `vuelos.indice` para escribirlo en Solr. Si algo falla, se reintenta y termina en `vuelos.indice.fallidos`. Suma una clave de ruteo a la 4.14 y una cola a la 4.19. La cola va aparte de `vuelos.eventos` para que, si Solr se cae, no se frenen los eventos de Pistas. Las dos instancias de Vuelos la consumen. Se descartaron dos opciones: que el relay escriba directo en Solr, porque habría que programar los reintentos aparte, y revisar MySQL cada 30 segundos, porque no es "a partir de eventos" (3.2). |
| 14.6 | Un cambio viejo no pisa uno nuevo | **Solr descarta una escritura con versión menor o igual a la que tiene** | Cada vuelo lleva su número de versión, que es el `data_version` del evento (4.10). Si llega tarde un evento viejo, si llega dos veces el mismo o si el comando de reconstrucción corre al mismo tiempo, Solr se queda con el dato más nuevo. Por eso la cola `vuelos.indice` no necesita un solo consumidor activo, como la 4.23. |
| 14.7 | Cuándo Solr muestra lo escrito | **En hasta 5 segundos** (agente) | Solr hace visible lo escrito cada 5 segundos. Así un cambio llega a la búsqueda bastante antes de los 60 segundos de la 14.2: se suma hasta 1 segundo del relay (4.5) y, si algo falla, 7 de reintentos (4.24). |
| 14.8 | Cómo encuentra la búsqueda por texto | **Por el comienzo, sin distinguir mayúsculas ni tildes** | "AR15" encuentra AR1550, "Bue" encuentra Buenos Aires y "cordoba" encuentra Córdoba. Se descartaron la palabra completa, porque obliga a escribir el número entero, y cualquier parte de la palabra, porque agranda el índice y trae resultados de más. |
| 14.9 | Fecha del filtro del tablero | **El día en hora de Argentina (UTC−3)** | Un vuelo de las 23:30 en Córdoba es de ese día, aunque en UTC sean las 02:30 del siguiente. Es lo que espera quien mira el tablero. El resto de los horarios de la API siguen en UTC. |
| 14.10 | Si Solr no responde al actualizar el índice | **Igual que los demás consumidores** | Se hacen 3 reintentos, a 1, 2 y 4 segundos (4.24), y el evento pasa a `vuelos.indice.fallidos`, con la alerta de la 11.4. Cuando Solr vuelve, se corre la reconstrucción (14.3). Se descartó reintentar sin límite, porque se comportaría distinto que el resto y los eventos se acumularían. |
| 14.11 | Cómo se corre la reconstrucción del índice (14.3) | **Un subcomando `reindex` del programa de Vuelos, que se corre con `docker compose run` sobre una instancia de Vuelos (con el #57, `vuelos-1`)** (Maxi, 9/11/2026; ver la 28) | No suma una ruta HTTP que habría que proteger en el gateway, y usa la misma configuración que el servicio. |
| 14.12 | Plazo de cada escritura a Solr | **2 segundos** (Maxi, 9/11/2026; ver la 28) | El mismo que usa Vuelos con Pistas (4.16). Con los 3 reintentos (14.10), un Solr caído tarda unos 13 segundos en mandar el evento a fallidos. |

Maxi eligió las decisiones 14.5 a 14.10 el 22/10/2026, al escribir el ADR-006, entre las opciones que propuso el agente. La 14.7 tiene la marca "(agente)" porque el valor de 5 segundos lo propuso el agente. Quedan confirmadas al mergearse el pull request #30.

## 15. API gateway

El 16/10/2026 el profe respondió si NGINX puede ser balanceador y gateway a la vez: *"De poder se puede hacer todo en un mismo archivo de configuración, de recomendarse es mejor tener dos archivos de configuración, uno para cada r…"*. El mensaje llegó cortado; se entiende "uno para cada rol".

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 15.1 | Con qué se hace | **A. NGINX** | El mismo NGINX del balanceo hace de gateway. No suma herramientas. |
| 15.2 | Tipo de token | **A. JWT, dura 1 hora** | El token lleva quién es el usuario y su rol, firmado. Se valida sin preguntarle a Usuarios: si Usuarios se cae, las sesiones abiertas siguen funcionando. |
| 15.3 | Cómo se separan los dos roles | **Un NGINX, dos archivos** | Un solo contenedor con `gateway.conf` (rutas, token y límite de tráfico) y `balanceo.conf` (las instancias de Vuelos y qué hacer si una se cae). Es lo que recomendó el profe. |
| 15.4 | Cómo verifica NGINX el token | **Un script con njs** | El NGINX gratuito no verifica JWT por sí solo; esa función es de la versión paga. njs es un módulo oficial de NGINX para escribir scripts cortos en JavaScript. El script verifica la firma y el vencimiento sin llamar a Usuarios, que es lo que buscaba la 15.2. |
| 15.5 | Firma del token | **HS256, con una clave compartida** (agente) | Usuarios firma y NGINX verifica con la misma clave, que sale del `.env`. Es la forma más simple de verificar desde el script. El token lleva el identificador del usuario, su rol, su aerolínea si es una, y el vencimiento. |
| 15.6 | Cómo reciben los servicios la identidad | **Encabezados que pone NGINX** (agente) | Después de verificar el token, NGINX manda `X-User-Id`, `X-User-Role` y, si es una aerolínea, `X-Airline`, y pisa los que vengan del cliente con esos nombres. El gateway autentica y cada servicio decide qué puede hacer ese usuario (clase 6). Mientras no esté el gateway, en las pruebas se mandan a mano. |
| 15.7 | Rutas | **Por prefijo** (agente) | `/api/vuelos/`, `/api/pistas/`, `/api/usuarios/` y `/api/clima/` van a cada servicio con el resto de la ruta. Sin token pasan solo el inicio de sesión y el tablero. El frontend se sirve desde el mismo NGINX, así no hay que configurar CORS. Cuando esté el gateway, solo NGINX expone un puerto. |
| 15.8 | Control de tráfico | **20 pedidos por segundo por IP** (agente) | Con ráfagas de hasta 40; al pasarlo responde `429`. El valor se configura: para las pruebas de carga del balanceo y la caché se sube, y una prueba aparte muestra el `429` con el valor normal. Con un solo NGINX no hay que coordinar el límite entre réplicas. |
| 15.9 | Valores de `X-User-Role` | **`airline` y `operator`** (Maxi, 21/10/2026) | Son los que ya usan Clima (#20) y el contrato de Vuelos (#18), en inglés como el resto de la API. Usuarios los pone en el token y NGINX los pasa en el encabezado (15.5, 15.6). Queda confirmada al mergearse el pull request #26, que revisa la pareja 2 porque vale para todos los servicios. |

Las decisiones 15.10 a 15.17 las tomó Male el 7/11/2026, al escribir el contrato de Usuarios (`services/usuarios/openapi.yaml`). Las opciones y las recomendaciones las preparó el agente de IA. Quedan confirmadas al mergearse el pull request que las registra, que revisa la pareja 1.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 15.10 | Ruta del inicio de sesión | **`POST /login`** | Por el gateway, `/api/usuarios/login` (15.7). Por qué: es la más clara para el frontend. Se descartó `POST /sessions` porque no se guarda ninguna sesión: el JWT no se registra en ningún lado. |
| 15.11 | Qué devuelve | **El token, y además `role`, `airline` y `expires_in`** | El frontend sabe qué pantalla mostrar sin abrir el token. `airline` va solo si el rol es `airline`. |
| 15.12 | Errores del inicio de sesión | **`401 INVALID_CREDENTIALS`, igual si el usuario no existe o la contraseña está mal** | Así no se puede averiguar qué usuarios existen. Además, `400 INVALID_REQUEST` si falta un dato y `500 INTERNAL_ERROR` ante un error inesperado. |
| 15.13 | Datos dentro del token | **`sub`, `role`, `airline`, `iat` y `exp`** | `sub`, `iat` y `exp` son los nombres estándar de JWT para el usuario, la emisión y el vencimiento. `airline` va solo para las aerolíneas. El script de NGINX (15.4) lee estos nombres. |
| 15.14 | Cómo se guarda la contraseña | **Hasheada con bcrypt** | Viene en `golang.org/x/crypto`, que ya está entre las dependencias. Es el estándar más conocido y el costo se ajusta solo. Se descartó Argon2id: más moderno, pero con más parámetros para elegir. |
| 15.15 | Cuentas de prueba | **Un operador y las aerolíneas `AR` y `LA`, con contraseñas del `.env`** | `AR` y `LA` son los códigos que ya usan los tests de Vuelos. Las contraseñas no quedan en el repo, que es público. |
| 15.16 | Cómo se conecta Usuarios a MySQL | **Usuario propio y la variable `MYSQL_DSN`, como Vuelos (5.9)** | Se suman `MYSQL_USUARIOS_USER` y `MYSQL_USUARIOS_PASSWORD` al `.env`. Usuarios no usa root. |
| 15.17 | Clave de firma del token | **La variable `JWT_SECRET` del `.env`** | La leen Usuarios para firmar y NGINX para verificar (15.5). |
| 15.22 | Nombres de las cuentas de prueba | **`operador`, `aerolineas` (AR) y `latam` (LA)** (Male, 9/11/2026) | Fáciles de recordar y de tipear en la presentación. Sus contraseñas son `USUARIOS_OPERADOR_PASSWORD`, `USUARIOS_AEROLINEAS_PASSWORD` y `USUARIOS_LATAM_PASSWORD` (15.15). Las opciones las preparó el agente; queda confirmada al mergearse el pull request de Usuarios. |

Las decisiones 15.18 a 15.21 las tomó Salvador el 9/11/2026, al programar el gateway (`nginx/`). Las opciones y las recomendaciones las preparó el agente de IA. Siguen la numeración de las 15.10 a 15.17 del contrato de Usuarios (#45). Quedan confirmadas al mergearse el pull request que las registra, que revisa la pareja 1.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 15.18 | Qué responde el gateway sin un token válido | **`401` con `INVALID_TOKEN`, en formato Problem Details** | Vale igual si falta el token, si no es válido o si venció: el cliente no necesita saber cuál, y el motivo queda en el log de NGINX. Lleva `WWW-Authenticate: Bearer`. Es el mismo formato de error que los servicios, así el frontend los maneja igual. Se descartó un `401` sin cuerpo. |
| 15.19 | La ruta interna de Pistas | **El gateway responde `404` a `/api/pistas/internal/`** | `POST /internal/slots` es la ruta con la que Vuelos le pide un slot a Pistas, y no controla quién pide. Si pasara por el gateway, cualquier aerolínea podría reservar un slot sin pasar por Vuelos. Por eso las rutas del operador en Pistas no van bajo `/internal`. El `/internal` de Clima sí pasa: ya pide el rol `operator`. Se descartó dejar pasar solo las rutas de los contratos, porque cada ruta nueva obligaría a tocar el gateway. |
| 15.20 | Cuándo dejan de exponer puerto los servicios | **Cuando Usuarios emita tokens** | La 15.7 dice que solo NGINX expone un puerto. Hoy Usuarios no entrega tokens, y la pareja 1 prueba contra los puertos directos con los encabezados a mano. Los puertos `8081` a `8084` se sacan en otro pull request, cuando Usuarios ande. Mientras tanto, `nginx/token-de-prueba.sh` arma tokens de prueba para el gateway y para k6. |
| 15.21 | Largo mínimo de `JWT_SECRET` | **32 bytes** | Es lo que pide HS256: una clave de al menos 256 bits (RFC 7518). Con una clave corta, cualquiera puede adivinarla probando y armar tokens válidos. Si es más corta, NGINX no arranca. Se genera con `openssl rand -base64 32`. |

## 16. Capacidad y costos (ADR D13)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 16.1 | Con qué se hacen las pruebas de carga | **A. k6** | Escenarios en JavaScript guardados en el repo, como la rampa de 10 a 1000 pedidos por segundo. Por qué: se integra con Grafana, que ya se usa (11.1). |

Las decisiones 16.2 a 16.5 las tomó Salvador el 9/11/2026, al escribir los escenarios de k6 (`tests/carga/`). Eran pendientes del [ADR-013](adr/ADR-013.md). Las opciones y las recomendaciones las preparó el agente de IA. Quedan confirmadas al mergearse el pull request que las registra, que revisa la pareja 1.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 16.2 | Escalones de la rampa del tablero | **10, 100, 500 y 1000 pedidos por segundo, un minuto cada uno, con un minuto de calentamiento** | Es la rampa que pidió el profe, y muestra en qué escalón se rompe. Cada corrida dura unos 5 minutos. Se descartaron más escalones de 30 segundos: ubican mejor el límite, pero los números salen más ruidosos. |
| 16.3 | Rampa del pedido de slot | **5, 20 y 50 pedidos por segundo, un minuto cada uno** | Las aerolíneas piden slots mucho menos que lo que se consulta el tablero, y cada pedido ocupa un slot real. Se descartó llegar a 200: llenaba bloques y los rechazos ensuciaban la medición. |
| 16.4 | Cómo se corre k6 | **Con Docker, desde el compose** | Un servicio `k6` con la imagen oficial que solo arranca si se lo pide (`--profile carga`). Nadie instala nada y le pega a NGINX por la red interna, igual en todas las computadoras. Se descartó instalar k6 en cada una. |
| 16.5 | Si los resultados de k6 van a Grafana | **Sí, por OpenTelemetry** | k6 manda sus métricas al mismo `grafana:4318` que los servicios, con `service_name="k6"`. Se ven los pedidos de k6 al lado del reparto entre instancias y la latencia de cada servicio. Además queda el resumen de k6 en un archivo. |

## 17. Bonus

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 17.1 | Cuáles se eligen | **Ninguno** | Se anuncia así en la Entrega 2. |

## 18. Reglas de negocio pendientes del SPEC

Son los puntos de la sección "Pendiente de definir" del [SPEC.md](../SPEC.md). La primera columna es el número de pendiente.

| Pendiente | Qué había que definir | Elegido |
|---|---|---|
| 1 | Cuánto dura un bloque | **15 minutos.** |
| 2 | Qué slots libres se informan al rechazar | **Los 3 más cercanos.** Antes o después del pedido, en cualquier pista y sin límite de distancia (agente). |
| 3 | Qué vuelos alcanza el cierre de una pista | **Todos los que tienen slot en esa pista de ahí en adelante** (agente). No hay que definir un período. El vuelo cuyo bloque ya empezó conserva su slot: pierden el slot los bloques que todavía no empezaron. Lo eligió Carola el 9/11/2026, al programar el cierre por clima. |
| 4 | Si se puede pedir un slot en una pista cerrada | **No.** |
| 5 | Si una pista cerrada a mano se reabre sola | **No: la reabre el operador.** |
| 6 | Cómo pasa un vuelo a "Finalizado" | **Automático, al pasar su bloque.** |
| 7 | Quién puede cancelar un vuelo | **La aerolínea dueña del vuelo** (agente). Un arribo desviado no se cancela: es un estado final. |
| 8 | Si se puede cambiar el slot de un vuelo programado | **No** (agente). Para cambiarlo se cancela y se publica de nuevo. Así no hace falta una operación que suelte un slot y tome otro a la vez. |
| 9 | Si cada aerolínea ve y opera solo sus vuelos | **Sí** (agente). Cada aerolínea opera solo los suyos; el tablero público muestra todos. |
| 10 | Cómo se crean las cuentas | **Se cargan al arrancar** (agente): un operador y dos aerolíneas de prueba. No hay registro. |
| 12 | Qué datos lleva un vuelo | **Los de F-02, más número de vuelo y ámbito** (agente). El número es, por ejemplo, `AR1550`; el ámbito es nacional o internacional. Los dos los usa la búsqueda (14.1). Origen y destino se guardan como nombre de ciudad. |
| 15 | Aeropuertos alternativos y quién los guarda | **San Luis (`SAOU`), Rosario (`SAAR`), Mendoza (`SAME`), Aeroparque (`SABE`) y Ezeiza (`SAEZ`). Los guarda Pistas.** El orden por distancia en línea recta desde Córdoba lo calculó el agente: unos 300, 370, 460, 650 y 660 km. |
| 16 | Qué pasa si ningún alternativo está disponible | **No se contempla:** se asume que siempre hay uno disponible. |
| 17 | Cómo se entera el sistema de que Clima no tiene datos | **Con un evento nuevo, "Clima sin datos"** (sección 4). Una condición forzada dura hasta que el operador la quita, y mientras dura manda sobre el dato real (agente). |
| 18 | Si el bloque pedido tiene que coincidir con el horario del vuelo | **No: se puede pedir cualquier bloque, y con slot vale el horario del bloque.** El tablero muestra, ordena y filtra por el horario del slot; sin slot, por el publicado. Así un vuelo demorado puede pedir un bloque más tarde. Apareció al escribir los endpoints de Vuelos; lo eligió Maxi el 19/10/2026 y queda confirmado al mergearse el pull request #17. |
| 19 | Si un vuelo puede tener el mismo origen y destino | **No, se rechaza al publicar.** Lo propuso Male en la revisión del pull request #19 y lo aceptó Maxi el 20/10/2026. |
| 20 | Si se puede publicar un vuelo con horario en el pasado | **No, se rechaza.** Se compara con el momento en que se publica; no afecta a los demorados, que ya están publicados. Lo propuso Male en la revisión del pull request #19 y lo aceptó Maxi el 20/10/2026. |
| 21 | Qué muestra el tablero si no se filtra por fecha y cómo se desempata al ordenar | **Los vuelos de hoy en adelante, en hora de Argentina (14.9). Los empates, por horario y después por número de vuelo.** Así el orden siempre es el mismo y la paginación no repite ni saltea vuelos. Lo eligió Maxi en el documento de cierre del 30/10/2026. |

El pendiente 11 se resolvió en la 14.1. El 14 se resolvió en la 27.1.

## 19. Reparto

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 19 | Cómo nos repartimos | **C. De a dos** | Carola con Maxi, y Male con Salvador. Las parejas las informó Maxi el 16/10/2026; qué hace cada una lo eligió el agente (ver "Reparto de la Entrega 2"). Por qué: para que todos entiendan todo. |

## 20. Para programar

Todo lo de esta sección lo eligió el agente, para que las dos parejas usen lo mismo.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 20.1 | Librerías comunes | Ver la tabla de abajo | Las mismas en todos los servicios. |
| 20.2 | Carpetas nuevas | `nginx/` y `tests/carga/` | En la raíz. `nginx/` tiene los dos archivos de configuración y el script del token; `tests/carga/`, los escenarios de k6. |
| 20.3 | Cobertura de tests unitarios | **60 % sobre las reglas de negocio** | Se mide sobre el código de reglas de Pistas y Vuelos, no sobre todo el repo. Se ajusta si el profe pide otro número. |
| 20.4 | Configuración de Grafana | **Una carpeta `grafana/` en la raíz** (Male, 7/11/2026) | Como `rabbitmq/` y `nginx/`. El `docker-compose.yml` la monta en el contenedor de Grafana, que carga al arrancar lo que hay adentro: hoy, la alerta de mensajes fallidos (11.4) en `grafana/alerting/`; después, el tablero. Se descartó guardarla en `docs/`, que es documentación. Las opciones las preparó el agente; queda confirmada al mergearse el pull request que la registra. |
| 20.5 | Configuración de Solr | **Una carpeta `solr/` en la raíz** (Maxi, 9/11/2026; ver la 28) | Como `rabbitmq/`, `nginx/` y `grafana/`: la monta el `docker-compose.yml`. Tiene el esquema del core y la cadena que descarta las versiones viejas (14.6). |

| Para qué | Librería |
|---|---|
| Logs | `log/slog`, la de Go |
| Trazas y métricas | OpenTelemetry para Go (`go.opentelemetry.io/otel`) |
| RabbitMQ | `github.com/rabbitmq/amqp091-go`, el cliente oficial |
| Circuit breaker (13.1) | `github.com/sony/gobreaker` |
| Límite de pedidos de Clima (9.6) | `golang.org/x/time/rate` |
| Memcached | `github.com/bradfitz/gomemcache` |
| Solr | Ninguna: llamadas HTTP a su API JSON |
| Firmar tokens en Usuarios | `github.com/golang-jwt/jwt/v5` |
| Tests de integración | `testcontainers-go`, que levanta una base real desde el test |

## Decisiones de la presentación

Las secciones 21 a 26, la fila 21 de la sección 18 y el uso de BMAD de "Forma de trabajo" salen del documento de cierre del proyecto del 30/10/2026. Las opciones y las recomendaciones las preparó el agente de IA; las elecciones las completó Maxi en el documento. El 9/11/2026, con la revisión de Male del pull request #43, Maxi corrigió la 21.1, la 25.1 y la 26.1, y eligió la 27.1 a partir del contrato del proveedor. Quedan confirmadas cuando el equipo apruebe el pull request que las registra.

## 21. Errores

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 21.1 | Qué responde un servicio ante un error inesperado | **`500` con `INTERNAL_ERROR` ante un error de programación, y `503` cuando una dependencia no responde** | `500` es para lo que no se arregla reintentando, como un pánico de un handler. `503` es para lo pasajero: en Pistas, que la base no responda o que un choque no se resuelva con los reintentos, como ya hace `responderError`; en Clima, la fuente sin datos. Así Vuelos sigue reintentando a Pistas ante `503` (4.5) y no reintenta un `500`, que igual cuenta como falla para el circuit breaker (13.2). Pistas ya responde así; falta anotar el `500` en su contrato (`services/pistas/openapi.yaml`). Lo marcó Male en la revisión del #43: la versión anterior pasaba todo a `500` y Vuelos dejaba de reintentar las caídas de la base. |

## 22. Frontend

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 22.1 | Quién hace cada pantalla | **Cada uno, las de su parte** | Maxi: el tablero público y la pantalla de la aerolínea (publicar, pedir slot, cancelar y ver sus vuelos). Carola: la del operador (pistas, cerrar y reabrir, forzar clima y el aviso de Clima sin datos). Male: el ingreso. Salvador: arma el proyecto y lo sirve desde NGINX (15.7). Por qué: nadie carga con todo y cada uno conoce la API de su parte. |
| 22.2 | Con qué se arma | **React con Vite** | React ya estaba elegido (7.2). Vite es la herramienta habitual hoy para arrancar un proyecto de React. |
| 22.3 | Cómo se ven iguales las pantallas | **Una base común, mergeada primero** | El menú y los estilos entran antes que las pantallas, así las cuatro partes usan lo mismo. |

Las decisiones 22.4 a 22.7 las tomó Salvador el 9/11/2026, al armar la base del frontend (`frontend/`). Las opciones y las recomendaciones las preparó el agente de IA. Quedan confirmadas al mergearse el pull request que las registra, que revisa la pareja 1.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 22.4 | Dónde va y cómo se pasa de una pantalla a otra | **`frontend/` en la raíz, con React Router** | Cada pantalla tiene su dirección (`/`, `/ingreso`, `/aerolinea`, `/operador`), así se puede recargar y compartir el link, y cada uno trabaja en su archivo de `src/paginas/` sin pisarse. React Router es la librería de ruteo más usada con React. Se descartó elegir la pantalla con un estado de React: al recargar se volvía al inicio. |
| 22.5 | Lenguaje | **JavaScript** | Quedan dos días y nadie del grupo programó el frontend todavía: sin tipos, cada uno arranca más rápido. Se descartó TypeScript, que ataja errores con los datos de la API pero suma tiempo. |
| 22.6 | Estilos comunes | **CSS propio, sin librerías** | Un archivo, `src/estilos.css`, con los colores y medidas en variables y clases para botones, formularios, tablas, avisos y el estado de un vuelo. Sin dependencias y fácil de explicar. Se descartaron una librería de componentes (hay que aprenderla y pesa) y Tailwind (mete los estilos en cada elemento). |
| 22.7 | Dónde guarda el frontend el token | **`sessionStorage`** | Sobrevive si se recarga la página y se borra al cerrar la pestaña; como el token dura 1 hora (15.2), queda poco tiempo expuesto. Un script malicioso en la página lo podría leer, igual que con `localStorage`, que además lo guardaría más tiempo. Se descartó tenerlo solo en memoria: al recargar había que volver a iniciar sesión. |

## 23. Contratos nuevos

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 23.1 | Cómo se definen el login, el tablero y las rutas del operador | **Como en la Entrega 2** | Quien programa el servicio propone su `openapi.yaml` en un pull request y lo revisa la otra pareja antes de programar. El frontend puede arrancar contra el contrato. |
| 23.2 | Rutas del operador en Pistas | **Bajo `/runways` y `/alternate-airports`, fuera de `/internal`** | En inglés y en plural, como `/flights` en Vuelos. Van fuera de `/internal` porque el gateway la bloquea (15.19); desde afuera quedan como `/api/pistas/runways` (15.7). Cubren F-09, F-10, F-11, F-20 y F-21. Solo las acepta el rol `operator` (CA-19); sin identidad responden `401` con `MISSING_IDENTITY` y con otro rol `403` con `FORBIDDEN_ROLE`, como Vuelos. El contrato está en `services/pistas/openapi.yaml`, versión 0.4.0. |
| 23.3 | Cómo se cierra y se reabre una pista | **`POST /runways/{id}/close` y `POST /runways/{id}/reopen`** | Una acción por ruta, fácil de leer. Los alternativos siguen la misma forma: `POST /alternate-airports/{icao}/disable` y `/enable`. Repetir una acción ya hecha responde `200` sin cambiar nada (agente). Se descartaron `PUT` y `DELETE` sobre `/closure`, como el forzado de Clima (9.13), y un `PATCH` sobre la pista. |
| 23.4 | Si el operador puede reabrir una pista que cerró el clima | **Solo cuando Clima no tiene datos** | Mientras haya datos, una pista cerrada por el clima la reabre el clima; si el operador intenta, se responde `409` con `RUNWAY_CLOSED_BY_WEATHER`. Sin datos, decide el operador (RN-24, CA-20). Una condición forzada cuenta como dato: mientras esté puesta, tampoco se reabre a mano, porque manda sobre todo (RN-29); lo preguntó Salvador en la revisión del #53. Para probar el cierre y la reapertura se usa el clima forzado (F-12). Queda en el SPEC como RN-42. Carola había elegido "siempre"; Male propuso esta en la revisión del #53 y Carola la aceptó el 9/11/2026. Se descartaron "siempre", que podía dejar una pista abierta con mal clima hasta el próximo cambio de aptitud, y "nunca", que contradecía CA-20. |
| 23.5 | Detalles del contrato del operador | **Los propuso el agente** | Si el operador cierra una pista que cerró el clima, pasa a cerrada por el operador y no se reabre sola (RN-16). Una pista nueva nace cerrada por el clima si el clima no permite operar (RN-15). `GET /runways` trae también el estado del clima, con el aviso de "Clima sin datos" (F-21), para que la pantalla haga un solo pedido. Se suman los errores `RUNWAY_ALREADY_EXISTS`, `RUNWAY_CLOSED_BY_WEATHER` y `ALTERNATE_NOT_FOUND`. |
| 23.6 | Si el operador puede reabrir una pista que cerró él mientras el clima no permite operar | **No, mientras Clima tenga datos** (Maxi, 9/11/2026) | Se responde el mismo `409` con `RUNWAY_CLOSED_BY_WEATHER`. Apareció en la revisión del #73: con tormenta, el operador cerraba a mano una pista que había cerrado el clima (pasaba a cerrada por el operador, 23.5) y después la reabría, así quedaba abierta con mal clima y se le podían dar slots, porque la asignación solo mira si la pista está abierta. Sin datos de Clima decide el operador, como en la 23.4. Se descartó dejarlo como limitación, porque esquivaba la RN-42. Queda en el SPEC en la RN-42 y el CA-42. Las opciones las preparó el agente; queda confirmada al mergearse el pull request que la registra. |

Las decisiones 23.2 a 23.5 las tomó Carola el 9/11/2026, al proponer el contrato del operador (23.1). Las opciones y las recomendaciones las preparó el agente de IA. Quedan confirmadas al mergearse el pull request que las registra, que revisa la pareja 2.

## 24. Caída provocada

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 24.1 | Qué se tira abajo para el informe de `docs/POSTMORTEM.md` | **Pistas, y después RabbitMQ** | Con Pistas caída se ve el circuit breaker de Vuelos (13.1): los vuelos quedan "pendiente de slot", y se ve en el frontend y en Grafana. Con RabbitMQ caído se ve que el outbox guarda los eventos y los publica cuando vuelve. Las dos van en el informe. |

## 25. Orden de los eventos

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 25.1 | Si los consumidores comparan `data_version` | **No** | En `vuelos.eventos` hay un solo consumidor activo (4.23), y Pistas corre en una sola instancia, así que lee `pistas.eventos` de a un consumidor: los eventos de un mismo relay llegan en orden. `pistas.eventos` no tiene `x-single-active-consumer`; importa porque ahí llegan también los avisos de aptitud de Clima, y un "no apto" viejo después de un "apto" nuevo dejaría la pista cerrada por error. Lo único que puede llegar desordenado son los eventos de las dos instancias de Vuelos (4.21), y hoy Pistas recibe de Vuelos solo "vuelo cancelado", una vez por vuelo y final: no hay dos eventos del mismo vuelo que se puedan pisar. Se revisa si Vuelos suma eventos hacia Pistas o si Pistas corre en más de una instancia. El índice de búsqueda sí compara (14.6). Cierra el pendiente del ADR-005. |

## 26. Fecha de cierre

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 26.1 | Cuándo tiene que estar todo en `main` | **Miércoles 11/11/2026, antes de presentar** | En el sorteo del 6/11 salió el 11/11, así que la presentación y el cierre son el mismo día. Antes era el lunes 9/11; se corrió para tener dos días más de trabajo, a costa de no tener un día aparte para ensayar. |
| 26.2 | Qué se asegura primero si no llega todo | **El tablero en el backend: Solr, `GET /board` y la caché con su medición** (Maxi, 9/11/2026; ver la 28) | El enunciado los pide con evidencia (2.6, 2.7). Las pantallas del tablero y de la aerolínea van después. |

## 27. Proveedor

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 27.1 | Qué capacidad del proveedor se consume y en qué flujo | **Las notificaciones por email de Recién hecho, para avisarle a la aerolínea cuando el clima le demora o le desvía un vuelo** | El proveedor asignado es Recién hecho, un ecommerce de alimentos, y solo ofrece mandar emails (`POST /notificaciones`). Cuando se cierran las pistas por el clima y un vuelo queda demorado o desviado (RN-17, RN-18), se le manda un mail a la aerolínea dueña. Por qué: tiene consecuencias de negocio, sale de un evento que ya existe y, como el envío es asincrónico, si el proveedor está dormido o falla no frena el cierre de las pistas. Cada envío lleva una `Idempotency-Key` armada con el vuelo y el evento, así un reintento no manda el mail dos veces. La clave del proveedor no va al repo: va en `.env` y en Render (9.5). Qué servicio lo manda, el mail de cada aerolínea, el timeout y los reintentos se definen en el ADR-009. Cierra el pendiente 14 del SPEC. |

Las decisiones 27.2 a 27.9 las tomó Male el 9/11/2026, al escribir el ADR-009 y programar el aviso, con las opciones y las recomendaciones del agente. La 27.2, la 27.3 y la 27.4 se consultaron al grupo por WhatsApp porque tocan Vuelos. Quedan confirmadas al mergearse el pull request que las registra, que revisa la pareja 1.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 27.2 | Qué servicio manda el mail | **Vuelos** | Es donde el vuelo pasa a demorado o desviado, cuando le llega "slot liberado por cierre" de Pistas, y sabe de qué aerolínea es. Se descartó Pistas, que no conoce a las aerolíneas. |
| 27.3 | Dónde está el mail de cada aerolínea | **La variable `AEROLINEAS_EMAILS` de Vuelos** | Con la forma `AR:mail,LA:mail`. No suma llamadas entre servicios. Se descartó guardarlo en Usuarios, que obligaba a Vuelos a llamarlo en cada aviso. |
| 27.4 | Cómo se manda sin frenar el cierre | **Un evento en el outbox, `vuelos.vuelo.afectado_por_cierre`, y un consumidor de Vuelos en la cola `vuelos.avisos`** | Se guarda en la misma transacción que deja el vuelo demorado o desviado: si el cambio se guarda, el aviso sale. Se descartó una llamada directa en segundo plano, que se pierde si Vuelos se reinicia. |
| 27.5 | Timeout y reintentos | **3 segundos por llamada y 6 reintentos con 2, 4, 8, 16, 32 y 64 segundos** | Unos 2 minutos, lo que recomienda el proveedor para despertarlo. Después, a `vuelos.avisos.fallidos` y la alerta de la 11.4. Sin circuit breaker: el envío no bloquea nada. |
| 27.6 | `Idempotency-Key` | **El id del evento** | Es único por aviso y no cambia entre reintentos: el proveedor no manda el mismo mail dos veces. |
| 27.7 | Test de contrato y doble para las pruebas | **Una copia de su contrato y su mock con Prism** | El contrato está en `services/vuelos/internal/adapters/notificaciones/testdata/`. El test levanta el mock con testcontainers y verifica que lo que mandamos lo cumple. El mismo mock corre en el Docker Compose (`notificaciones-mock`) para el desarrollo y las pruebas de carga; el real se usa en la presentación, con `NOTIFICACIONES_URL` en el `.env`. |
| 27.8 | Qué cierres avisan | **Todos, del clima o del operador** | El evento de Pistas no dice el motivo del cierre. El mail dice "por el cierre de la pista", sin nombrar el clima. |
| 27.9 | Qué ve el usuario si el proveedor falla | **El vuelo cambia igual; solo el mail llega tarde o no llega** | La aerolínea ve el vuelo demorado o desviado en su pantalla y en el tablero. Si se agotan los reintentos, el aviso queda en fallidos y la alerta avisa al equipo. |

## 28. Vuelos para la presentación

Las decisiones 10.5 a 10.8, 14.11, 14.12, 20.5, 26.2 y 28.1 salen de lo que quedaba pendiente para programar la búsqueda, la caché y el finalizado automático (ADR-006 y ADR-007). Las opciones y las recomendaciones las preparó el agente de IA; Maxi eligió las recomendadas el 9/11/2026 y Salvador estuvo de acuerdo en el grupo. Quedan confirmadas cuando el equipo apruebe el pull request que las registra.

Las 10.9 y 10.10 aparecieron al preparar la medición de la caché: hacían falta datos en el índice y el ensayo mostró que los filtros de k6 se repetían demasiado. Las opciones las preparó el agente de IA y Maxi eligió las recomendadas el 9/11/2026. Quedan confirmadas cuando el equipo apruebe el pull request que las registra.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 28.1 | Cada cuánto se dan por finalizados los vuelos (RN-32, F-22) | **Cada 1 minuto** | Vuelos busca los vuelos programados cuyo bloque ya terminó y los pasa a finalizado, cada uno con su "vuelo actualizado" para el tablero (14.5). Con bloques de 15 minutos (RN-03), un minuto de atraso casi no se nota, y la consulta es liviana. |

## Forma de trabajo

| Decisión | Elegido | Qué significa |
|---|---|---|
| Formato de los ADR | **A** | Formato simple: título, fecha, estado, contexto, alternativas evaluadas, decisión, consecuencias aceptadas y a cuál reemplaza. |
| Cómo se mergea | **A** | Con commit de merge, conservando los commits de la rama. |
| Dónde vive el alcance | `SPEC.md` en la raíz | Un solo archivo con funcionalidades, reglas de negocio y criterios de aceptación. Es el documento que manda. Decidido el 8/10/2026. |
| Uso de BMAD | **Sí, para las tareas de la presentación** | Para la Entrega 1 la documentación se escribe directo. Cuando arranque el desarrollo, se le pasa a BMAD todo lo producido (`SPEC.md`, decisiones, arquitectura y ADR) y su spec se genera a partir de eso. Si algo cambia, se cambia primero el documento original y se vuelve a generar el de BMAD; nunca al revés. Propuesto por Maxi el 8/10/2026; lo eligió Maxi en el documento de cierre del 30/10/2026. |

## Reparto de la Entrega 1

Decidido el 8/10/2026. La Entrega 1 vence el viernes 9/10. Cada paquete se trabaja en una rama propia y entra por pull request.

| Paquete | Quién | Qué produce | Decisiones de las que sale | Lo revisa |
|---|---|---|---|---|
| **Alcance** | Maxi | Documento de alcance (roles, funcionalidades, reglas de negocio, estados, criterios de aceptación) y `README.md` | 1 y 2.1 a 2.7 | Male |
| **Arquitectura** | Carola | `docs/ARCHITECTURE.md`, diagramas de contexto y de contenedores, ADR D1 | 3.1 a 3.3 | Salvador |
| **Clima hacia afuera** | Male | Contrato OpenAPI, su documentación con ejemplos y errores, el mock, ADR D8 | 6.1 a 6.19 | Maxi |
| **Datos, comunicación y esqueleto** | Salvador | ADR D3, ADR D5 y las carpetas iniciales de los cuatro servicios | 4.1 a 4.9, 5.1 a 5.5 y 7 | Carola |

Los pares de revisión son una sugerencia; el equipo todavía no los confirmó.

El detalle de qué tiene que hacer cada uno está en [entrega-1.md](entrega-1.md).

## Reparto de la Entrega 2

La Entrega 2 vence el viernes 23/10/2026. El trabajo de a dos lo decidió el equipo (19), las parejas las informó Maxi y qué hace cada pareja lo eligió el agente. Cada pull request lo revisa alguien de la otra pareja.

| Pareja | Quiénes | Qué produce | Decisiones de las que sale |
|---|---|---|---|
| **Vuelos y Pistas** | Carola y Maxi | El flujo principal con la operación crítica, `SPEC.md` 0.3, ADR-002, ADR-006 y ADR-010, validación de ADR-001 y ADR-005, y `ARCHITECTURE.md` al día | 2, 4, 8.1, 12.3, 13.1, 14, 15.6, 18 y 20 |
| **Clima y lo transversal** | Male y Salvador | Clima real publicado en la nube, Grafana en el Docker Compose, ADR-007, ADR-009, ADR-011, ADR-012 y ADR-013, ADR-008 al día y validación de ADR-003 | 6, 9, 10, 11, 12, 15.3, 16 y 20 |

El detalle está en [entrega-2.md](entrega-2.md).

## Reparto de la presentación

La presentación grupal es el miércoles 11/11/2026 (26.1). Se mantienen las parejas (19) y cada uno sigue con lo que hizo en la Entrega 2. Cada pull request lo revisa alguien de la otra pareja.

| Quién | Qué produce |
|---|---|
| Carola | Pistas y el clima (cerrar y reabrir pistas, cierre manual, demoras y desvíos), el `500` en el contrato de Pistas (21.1), ADR-004, la caída provocada y `docs/POSTMORTEM.md` (24.1), `docs/ARCHITECTURE.md` con limitaciones y deuda técnica, y la pantalla del operador |
| Maxi | La búsqueda con Solr, la caché con Memcached y su medición, `README.md` final, lo pendiente de Vuelos y de este documento, y las pantallas del tablero y de la aerolínea |
| Male | Usuarios (inicio de sesión y tokens), la capacidad del proveedor (27.1) con su test de contrato y el ADR-009, la observabilidad completa (tablero, alerta y objetivo de servicio) con el ADR-011, el `500` del contrato de Clima y la pantalla de ingreso |
| Salvador | El gateway con NGINX y el ADR-014, la segunda instancia de Vuelos con el balanceo, las pruebas de carga con k6 y el ADR-013, y el proyecto del frontend servido desde NGINX |
| Todos | Los tests de su propio servicio, con la cobertura de la 20.3 |

Lo primero, porque traba a otros: el gateway, Usuarios, la base del frontend (22.3) y los contratos nuevos (23.1).

## Lo que sigue abierto

Los nueve puntos que había dejado abiertos la Entrega 1 se cerraron en la Entrega 2: caché (10.2), hosting y mock (9.4, 9.7 y 9.8), gateway (15), Clima sin datos y aeropuertos alternativos (18), valores (18, 10.3 y 14.2), límite de pedidos de Clima (9.6), mensajes fallidos (11.4) y los temas de la Entrega 2 (secciones 10 a 17).

El ADR D4, que no tenía entrega asignada, se escribió para la presentación: es el [ADR-004](adr/ADR-004.md) (#63). No queda nada abierto.

Lo que se consume del proveedor quedó en la 27.1; el resto se define en el ADR-009.

Los endpoints de Vuelos y Pistas quedaron en sus contratos (4.14 a 4.16 y `services/vuelos/openapi.yaml`), y los nuevos siguen la 23.1. El uso de BMAD se confirmó ("Forma de trabajo").

## Puntos a cuidar

Consecuencias de lo elegido que conviene tener presentes al diseñar:

- **La prioridad de los arribos (2.7) complica la operación crítica.** Asignar un slot a un arribo puede quitárselo a un despegue. Sacar un vuelo y poner otro tiene que ocurrir como una sola operación, aun con pedidos simultáneos, y generar el evento "slot cedido a un arribo" sin perderlo. Es el centro del ADR D4.
- **Pistas trabaja con el último clima conocido (4.2).** Si un aviso de Clima se demora, Pistas puede asignar un slot en una pista que debería estar cerrada. El atraso aceptado es de hasta 30 segundos, como valor inicial (4.5).
- **Instancia separada (5.2) suma servidores.** Con MySQL para Vuelos, MySQL para Pistas, MongoDB para Clima y la base de Usuarios, más Solr, RabbitMQ y la caché, el arranque local levanta muchos contenedores. Conviene verificar que las computadoras del equipo lo soporten.
- **Vuelos reúne varias cosas:** guarda vuelos, mantiene el índice de búsqueda y aplica CQRS. Es el servicio con más piezas.
- **El tablero puede tardar hasta 2 minutos en mostrar un cambio:** hasta 60 segundos del índice (14.2) más hasta 60 de la caché (10.3). Hay que poder explicar por qué se acepta.
- **Las dos copias de Clima (9.3) pueden mostrar datos distintos** durante unos minutos, porque cada una consulta a Open-Meteo por su cuenta.
- **Render duerme el servicio (9.4):** el primer pedido después de un rato sin uso tarda cerca de un minuto, y puede traer un dato viejo o un `503`. Hay que avisarlo en la guía del contrato.
- **Los servicios confían en los encabezados de identidad (15.6).** Cuando esté el gateway, solo se tiene que poder llegar a ellos a través de NGINX (15.7).
- **El circuit breaker vive en cada instancia de Vuelos (13.1 y 12.1):** cada una decide por su cuenta si deja de llamar a Pistas.

## Preguntas para el profe

Respondida:

- *Con qué se hace el API gateway, y si puede ser el mismo NGINX del balanceo (7.4).* Sí: se puede todo en un archivo, y recomienda dos, uno por rol (16/10/2026). Ver la sección 15.

Sin respuesta registrada:

- Cuál es el formato de ADR: el enunciado remite a una sección 8 que no existe.
- Si el servicio que publica la capacidad (Clima) cuenta entre los tres microservicios mínimos.
- En qué entrega vence la decisión D4.
- Si el mock tiene que estar publicado en internet para el 9/10.
- Si hay que agregar a los profes como colaboradores del repo, ahora que es público.
- Cuándo es la jornada de fallas controladas.
- Qué porcentaje de cobertura de tests unitarios esperan. Mientras tanto, se usa el de la 20.3.
- Si la API pública de Clima puede no pasar por nuestro gateway (9.9).
- Si el Clima de la nube puede ser una copia separada del que usa nuestro sistema (9.3).
- Cuándo es la clase de observabilidad.

## A qué ADR va cada decisión

| ADR | Tema | Decisiones que lo alimentan | Vence |
|---|---|---|---|
| D1 | Límites de los servicios | 3.1, 3.2, 3.3 | Entrega 1 |
| D8 | Contrato propio | 6.1 a 6.19; se actualiza con 9.2, 9.4 y 9.6; la 9.10 lo reemplaza por el ADR-015 | Entrega 1; se actualiza en la 2 |
| D3 | Persistencia | 5.1 a 5.5, 7.2 | Entrega 1, versión inicial; se valida en la 2 |
| D5 | Comunicación entre servicios | 4.1 a 4.9, 25.1 | Entrega 1, versión inicial; se valida en la 2 |
| D2 | Arquitectura interna | 7.3, 7.5, 7.6, 20 | Entrega 2 |
| D4 | Consistencia y concurrencia | 2.3, 2.7, 4.4, 4.6, 4.9, 5.7, 23.6 | Presentación (ADR-004, #63) |
| D6 | Búsqueda | 3.2, 7.2, 10.4, 14.1 a 14.12, 20.5 | Entrega 2 |
| D7 | Caché | 10.1 a 10.8 | Entrega 2 |
| D9 | Consumo del proveedor | 27.1 | Entrega 2; se completa para la presentación |
| D10 | Resiliencia | 4.5, 13.1, 21.1, 24.1 | Entrega 2 |
| D11 | Observabilidad | 11.1 a 11.5 | Entrega 2, primera versión |
| D12 | Balanceo de carga | 12.1 a 12.3, 15.3 | Entrega 2 |
| D13 | Capacidad y costos | 11.3, 16.1 | Entrega 2 |
| ADR-014 | API gateway (adicional, agente) | 15.1 a 15.8 | Presentación |
