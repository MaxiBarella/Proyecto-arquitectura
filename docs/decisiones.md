# Decisiones del equipo

Registro de lo que el equipo decidió sobre el sistema. Es la fuente de la que salen los ADR, la documentación de alcance y la de arquitectura.

- **Cuándo:** reunión del 7/10/2026, cerrada la madrugada del 8/10.
- **Cómo:** el equipo completó el documento de decisiones (un Word con opciones por cada punto) y este archivo transcribe lo que eligieron.
- **Numeración:** es la del documento de decisiones.

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

| Evento | Quién lo publica | Quién lo recibe y para qué |
|---|---|---|
| Slot asignado | Pistas | Vuelos: pasa el vuelo a "programado" y actualiza el tablero. |
| Slot liberado por cierre | Pistas | Vuelos: pasa el despegue a "demorado" y el arribo a "desviado". Actualiza el tablero. |
| Slot cedido a un arribo | Pistas | Vuelos: pasa a "demorado" el despegue que perdió su slot. |
| Pista cerrada / reabierta | Pistas | Vuelos: para mostrarlo en el tablero. |
| Aptitud operativa cambió | Clima | Pistas: cierra o reabre las pistas. |
| Vuelo cancelado | Vuelos | Pistas: libera el slot. |

## 5. Datos

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 5.1 | Tipo de base por servicio | **A** | Vuelos y Pistas, relacional. Clima, no relacional. |
| 5.2 | Cuántas bases | **A** | Instancia separada: cada servicio levanta su propio servidor de base de datos. |
| 5.3 | Base del servicio Usuarios | **MySQL, instancia propia** | Con una restricción única sobre el nombre de usuario. Las contraseñas se guardan hasheadas. |
| 5.4 | Acceso a las bases | **GORM y driver oficial** | Vuelos, Pistas y Usuarios acceden a MySQL con GORM. Clima usa el driver oficial de MongoDB para Go. |
| 5.5 | Estructuras iniciales | Las del ADR-003 | Pistas: pistas, slots con restricción única sobre pista y bloque, outbox y mensajes procesados. Vuelos: vuelos, outbox y mensajes procesados. Clima: observaciones y un documento de aptitud por aeropuerto. Usuarios: usuarios. |

Las decisiones 5.3 a 5.5 las tomó Salvador el 8/10/2026, al escribir el [ADR-003](adr/ADR-003.md). Quedaron confirmadas al mergearse el pull request #8.

## 6. Clima hacia afuera

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 6.1 | Qué ofrece | **B** | Condiciones actuales por aeropuerto y aptitud operativa según umbrales propios. Sin avisos por evento hacia afuera. |
| 6.2 | Origen de los datos | **C** | Datos reales, con la posibilidad de que el operador fuerce una condición. |
| 6.3 | Formato del contrato | **A** | API descrita con OpenAPI. |
| 6.4 | Quién puede usarla | **B** | Una clave por consumidor, enviada en cada pedido. |
| 6.5 | Versionado | **A** | La versión va en la dirección (`/v1/...`). Los cambios compatibles no cambian la versión; los que rompen crean una nueva y la anterior se mantiene. |
| 6.6 | Mock | **A** | Generado a partir del archivo del contrato. |

Las decisiones 6.7 a 6.19 surgieron al preparar el contrato de Clima. Las tomó Male el 8/10/2026 y quedaron confirmadas al mergearse el pull request #7.

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 6.7 | Forma de la aptitud | **Una por aeropuerto** | Clima informa si se puede operar o no, sin separar aterrizaje de despegue ni distinguir pistas. Si no se puede, se cierran todas las pistas para las dos cosas (2.2, 2.4 y 2.5). |
| 6.8 | Fuente real | **Open-Meteo** | La del práctico de la clase 7. Trabaja con coordenadas, así que Clima guarda la latitud y longitud de nuestro aeropuerto. Forzar una condición (6.2) sirve para probar el cierre de pistas. |
| 6.9 | Aeropuertos atendidos | **Solo el nuestro** | La API atiende únicamente Córdoba (`SACO`, decisión 2.8). Los aeropuertos alternativos de la 2.9 no pasan por Clima. |
| 6.10 | Identificador del aeropuerto | **Código ICAO** | Por ejemplo `SACO`. |
| 6.11 | Umbrales de aptitud | **Simplificados** | No se opera con viento sostenido de más de 50 km/h, ráfagas de más de 65 km/h, visibilidad de menos de 800 m o tormenta eléctrica. |
| 6.12 | Rutas | **Un endpoint** | `GET /v1/airports/{icao}/conditions` devuelve las condiciones y la aptitud juntas. Forzar una condición es una ruta interna que no se publica. |
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
| 7.4 | API gateway | **Pendiente** | Se le pregunta al profe. |
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
| Arranque local | Docker Compose |
| Caché | **Sin cerrar:** la tabla decía "Redis o Memcached". |
| Observabilidad | Se decide después de la clase del tema. |
| Hosting de Clima | **Sin cerrar:** gratuito y simple; falta elegir cuál. |

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

## Lo que estas decisiones dejan abierto

Elecciones que aparecen por lo que se decidió y que todavía nadie tomó:

1. **Caché:** Redis o Memcached (7.2).
2. **Hosting de Clima y del mock:** qué proveedor (7.2) y si el mock se publica (6.17).
3. **API gateway:** con qué se hace; depende de la respuesta del profe (7.4).
4. **Clima sin datos:** cómo se entera Pistas de que Clima no puede obtener el clima, por ejemplo con un evento nuevo (6.19, D5).
5. **Aeropuertos alternativos:** cuáles forman la lista de la 2.9 y dónde se guarda.
6. **Valores:** duración del slot, vigencia de la caché y retraso tolerable del índice. Los timeouts y reintentos tienen valores iniciales (4.5) que se ajustan con las pruebas de carga.
7. **Límite de pedidos de Clima:** si se implementa en Clima o en el gateway (6.15, 7.4).
8. **Mensajes fallidos:** quién revisa la cola y con qué alerta; se define con la observabilidad (D11).
9. **Para la Entrega 2:** qué se cachea (D7), qué servicio se balancea (D12), qué mecanismo de resiliencia se implementa (D10), observabilidad (D11) y los bonus.

## Puntos a cuidar

Consecuencias de lo elegido que conviene tener presentes al diseñar:

- **La prioridad de los arribos (2.7) complica la operación crítica.** Asignar un slot a un arribo puede quitárselo a un despegue. Sacar un vuelo y poner otro tiene que ocurrir como una sola operación, aun con pedidos simultáneos, y generar el evento "slot cedido a un arribo" sin perderlo. Es el centro del ADR D4.
- **Pistas trabaja con el último clima conocido (4.2).** Si un aviso de Clima se demora, Pistas puede asignar un slot en una pista que debería estar cerrada. El atraso aceptado es de hasta 30 segundos, como valor inicial (4.5).
- **Instancia separada (5.2) suma servidores.** Con MySQL para Vuelos, MySQL para Pistas, MongoDB para Clima y la base de Usuarios, más Solr, RabbitMQ y la caché, el arranque local levanta muchos contenedores. Conviene verificar que las computadoras del equipo lo soporten.
- **Vuelos reúne varias cosas:** guarda vuelos, mantiene el índice de búsqueda y aplica CQRS. Es el servicio con más piezas.

## Preguntas para el profe

- Con qué se espera que se haga el API gateway (7.4).
- Cuál es el formato de ADR: el enunciado remite a una sección 8 que no existe.
- Si el servicio que publica la capacidad (Clima) cuenta entre los tres microservicios mínimos.
- En qué entrega vence la decisión D4.
- Si el mock tiene que estar publicado en internet para el 9/10.
- Si hay que agregar a los profes como colaboradores del repo, ahora que es público.
- Cuándo es la jornada de fallas controladas.
- Qué porcentaje de cobertura de tests unitarios esperan.

## A qué ADR va cada decisión

| ADR | Tema | Decisiones que lo alimentan | Vence |
|---|---|---|---|
| D1 | Límites de los servicios | 3.1, 3.2, 3.3 | Entrega 1 |
| D8 | Contrato propio | 6.1 a 6.19 | Entrega 1 |
| D3 | Persistencia | 5.1 a 5.5, 7.2 | Entrega 1, versión inicial |
| D5 | Comunicación entre servicios | 4.1 a 4.9 | Entrega 1, versión inicial |
| D2 | Arquitectura interna | 7.3, 7.5, 7.6 | Entrega 2 |
| D4 | Consistencia y concurrencia | 2.3, 2.7, 4.4 | Sin entrega asignada |
| D6 | Búsqueda | 3.2 | Entrega 2 |
