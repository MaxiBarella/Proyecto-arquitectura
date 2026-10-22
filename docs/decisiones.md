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

La 4.14 la tomó Maxi el 21/10/2026 y queda confirmada al mergearse el pull request #26. Las claves de Pistas las eligió por Carola, que le delegó la elección.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 4.14 | Claves de ruteo de los eventos | **Servicio, cosa y qué pasó, en español, como las de Clima (9.14)** | `vuelos.vuelo.cancelado`; `pistas.slot.asignado`, `pistas.slot.cedido`, `pistas.slot.liberado_por_cierre`, `pistas.pista.cerrada` y `pistas.pista.reabierta`. Así un consumidor puede pedir todo lo de un servicio con `pistas.#`. |

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

## 10. Caché (ADR D7)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 10.1 | Qué se cachea | **A. El tablero público** | Las consultas del tablero, en Vuelos. Se mide primero sin caché para tener el número de "antes". Por qué: es lo más leído del sistema. |
| 10.2 | Herramienta | **Memcached** | Una sola, compartida por las dos instancias de Vuelos (12.1). Por qué: se usó en el práctico de la clase 3. |
| 10.3 | Cómo se mantiene al día | **A. Solo vencimiento, 60 segundos** | Cada entrada dura 60 segundos y después se vuelve a calcular. No se borra cuando llega un evento. Por qué: con un vencimiento corto, borrar por evento casi no cambia nada. |
| 10.4 | Si Memcached no responde | **Se consulta Solr directo** (agente) | El tablero sigue funcionando, más lento, y queda un log del error. La caché nunca es imprescindible para responder. |

## 11. Observabilidad (ADR D11)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 11.1 | Dónde se ven logs, métricas y trazas | **A. Grafana en un contenedor** | Los servicios emiten todo con OpenTelemetry hacia una sola imagen que trae Grafana y sus almacenes ya configurados (`grafana/otel-lgtm`). Por qué: menos contenedores y menos peso en el arranque local. |
| 11.2 | Cómo se correlaciona un pedido | **A. El identificador de la traza** | El que genera OpenTelemetry. En HTTP viaja solo; en los eventos de RabbitMQ se copia en el campo de correlación que ya tiene cada mensaje (4.8). Por qué: un solo identificador para logs y trazas. |
| 11.3 | Objetivo de servicio | **Pedido de slot: el 95 % en menos de 500 ms** | Medido sobre los últimos 5 minutos. Es una primera versión: todo corre en una sola computadora y en los picos de la prueba de carga algunos pedidos van a tardar más. |
| 11.4 | Mensajes fallidos (pendiente de la Entrega 1) | **Log de error y alerta en Grafana** (agente) | Cuando un consumidor manda un mensaje a la cola de fallidos, escribe un log de error con el identificador de correlación, y Grafana alerta cuando aparece uno. Se revisan a mano desde el panel de RabbitMQ. Sirve como la alerta que pide el ADR D11. |
| 11.5 | Formato de los logs | **JSON con `log/slog`** (agente) | Es la librería de logs que trae Go. Cada línea lleva el identificador de la traza. |

## 12. Balanceo de carga (ADR D12)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 12.1 | Qué servicio se balancea | **A. Vuelos** | Dos instancias de Vuelos detrás de NGINX. Por qué: es el que más tráfico recibe y tiene el tablero cacheado, así que caché y balanceo se muestran sobre el mismo flujo. |
| 12.2 | Cómo reparte | **A. Por turnos** | El comportamiento por defecto de NGINX. Una instancia caída se detecta de forma pasiva, porque la verificación activa es de la versión paga; el ADR explica por qué alcanza. Por qué: es lo más simple. |
| 12.3 | Eventos duplicados con dos instancias | **Se aceptan** (agente) | Las dos instancias revisan la misma tabla de outbox y pueden publicar dos veces un evento. No rompe nada porque los consumidores son idempotentes (4.9), y no hay que programar nada extra. Las dos leen de la misma cola, así que cada evento que le llega a Vuelos lo procesa una sola. |

## 13. Resiliencia (ADR D10)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 13.1 | Dónde va el mecanismo de protección | **B. Circuit breaker de Vuelos a Pistas** | Si Pistas viene fallando, Vuelos deja de llamarla un rato y los vuelos quedan "pendiente de slot". Se suma al timeout y los reintentos de la 4.5. Por qué: el efecto se ve en el frontend. |

## 14. Búsqueda (ADR D6)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 14.1 | Filtros y orden del tablero | Ver al lado | Filtros: tipo de operación, estado, aerolínea y fecha (agente: el documento quedó sin marcar y son los recomendados). Búsqueda por texto: número de vuelo y ciudad. Orden por defecto: horario. Otros órdenes: aerolínea y ámbito, nacional o internacional. |
| 14.2 | Atraso máximo del índice | **60 segundos** | Desde que cambia un vuelo hasta que se ve en la búsqueda. Por qué: es una espera razonable para un tablero. |
| 14.3 | Cómo se reconstruye el índice | **A. Comando manual** | Lee los vuelos de MySQL y los vuelve a cargar en Solr. Por qué: si fuera automático al arrancar, las dos instancias de Vuelos lo harían a la vez. |
| 14.4 | Paginación | **20 vuelos por página** (agente) | El enunciado pide paginación sin dar un valor. |

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

## 16. Capacidad y costos (ADR D13)

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 16.1 | Con qué se hacen las pruebas de carga | **A. k6** | Escenarios en JavaScript guardados en el repo, como la rampa de 10 a 1000 pedidos por segundo. Por qué: se integra con Grafana, que ya se usa (11.1). |

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
| 3 | Qué vuelos alcanza el cierre de una pista | **Todos los que tienen slot en esa pista de ahí en adelante** (agente). No hay que definir un período. |
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

El pendiente 11 se resolvió en la 14.1. El 14 depende de la capacidad que asigne la cátedra.

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

## Forma de trabajo

| Decisión | Elegido | Qué significa |
|---|---|---|
| Formato de los ADR | **A** | Formato simple: título, fecha, estado, contexto, alternativas evaluadas, decisión, consecuencias aceptadas y a cuál reemplaza. |
| Cómo se mergea | **A** | Con commit de merge, conservando los commits de la rama. |
| Dónde vive el alcance | `SPEC.md` en la raíz | Un solo archivo con funcionalidades, reglas de negocio y criterios de aceptación. Es el documento que manda. Decidido el 8/10/2026. |
| Uso de BMAD | Al empezar a programar | Para la Entrega 1 la documentación se escribe directo. Cuando arranque el desarrollo, se le pasa a BMAD todo lo producido (`SPEC.md`, decisiones, arquitectura y ADR) y su spec se genera a partir de eso. Si algo cambia, se cambia primero el documento original y se vuelve a generar el de BMAD; nunca al revés. Propuesto por Maxi el 8/10/2026; falta que lo confirme el resto del equipo. |

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

## Lo que sigue abierto

Los nueve puntos que había dejado abiertos la Entrega 1 se cerraron en la Entrega 2: caché (10.2), hosting y mock (9.4, 9.7 y 9.8), gateway (15), Clima sin datos y aeropuertos alternativos (18), valores (18, 10.3 y 14.2), límite de pedidos de Clima (9.6), mensajes fallidos (11.4) y los temas de la Entrega 2 (secciones 10 a 17).

Queda abierto:

1. **El proveedor y el consumidor:** qué capacidad de otro grupo consumimos y qué grupo consume nuestro Clima. Lo asigna la cátedra y frena el ADR D9 y el pendiente 14 del SPEC.
2. **El ADR D4:** en qué entrega vence.
3. **Los endpoints de Vuelos y Pistas:** cada pareja propone los suyos en su pull request y los revisa la otra.
4. **El uso de BMAD:** la propuesta de Maxi de la sección "Forma de trabajo" sigue sin confirmar.

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
- Qué grupo consume nuestro Clima y qué capacidad nos toca consumir.
- Cuándo es la clase de observabilidad.

## A qué ADR va cada decisión

| ADR | Tema | Decisiones que lo alimentan | Vence |
|---|---|---|---|
| D1 | Límites de los servicios | 3.1, 3.2, 3.3 | Entrega 1 |
| D8 | Contrato propio | 6.1 a 6.19; se actualiza con 9.2, 9.4 y 9.6; la 9.10 lo reemplaza por el ADR-015 | Entrega 1; se actualiza en la 2 |
| D3 | Persistencia | 5.1 a 5.5, 7.2 | Entrega 1, versión inicial; se valida en la 2 |
| D5 | Comunicación entre servicios | 4.1 a 4.9 | Entrega 1, versión inicial; se valida en la 2 |
| D2 | Arquitectura interna | 7.3, 7.5, 7.6, 20 | Entrega 2 |
| D4 | Consistencia y concurrencia | 2.3, 2.7, 4.4 | Sin entrega asignada |
| D6 | Búsqueda | 3.2, 14.1 a 14.4 | Entrega 2 |
| D7 | Caché | 10.1 a 10.4 | Entrega 2 |
| D9 | Consumo del proveedor | Depende de la asignación de la cátedra | Entrega 2 |
| D10 | Resiliencia | 4.5, 13.1 | Entrega 2 |
| D11 | Observabilidad | 11.1 a 11.5 | Entrega 2, primera versión |
| D12 | Balanceo de carga | 12.1 a 12.3, 15.3 | Entrega 2 |
| D13 | Capacidad y costos | 11.3, 16.1 | Entrega 2 |
| ADR-014 | API gateway (adicional, agente) | 15.1 a 15.8 | Presentación |
