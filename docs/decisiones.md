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

## 6. Clima hacia afuera

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 6.1 | Qué ofrece | **B** | Condiciones actuales por aeropuerto y aptitud operativa según umbrales propios. Sin avisos por evento hacia afuera. |
| 6.2 | Origen de los datos | **C** | Datos reales, con la posibilidad de que el operador fuerce una condición. |
| 6.3 | Formato del contrato | **A** | API descrita con OpenAPI. |
| 6.4 | Quién puede usarla | **B** | Una clave por consumidor, enviada en cada pedido. |
| 6.5 | Versionado | **A** | La versión va en la dirección (`/v1/...`). Los cambios compatibles no cambian la versión; los que rompen crean una nueva y la anterior se mantiene. |
| 6.6 | Mock | **A** | Generado a partir del archivo del contrato. |

## 7. Tecnologías

| # | Decisión | Elegido | Qué significa |
|---|---|---|---|
| 7.1 | Un lenguaje o varios | **A** | El mismo para todos los servicios: Go con Gin. |
| 7.2 | Las piezas | Las sugeridas en la tabla | Ver abajo. |
| 7.3 | Patrones internos | **B** | Pistas, Hexagonal. Vuelos, CQRS. Clima, en capas. |
| 7.4 | API gateway | **Pendiente** | Se le pregunta al profe. |

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

## Reparto de la Entrega 1

Decidido el 8/10/2026. La Entrega 1 vence el viernes 9/10. Cada paquete se trabaja en una rama propia y entra por pull request.

| Paquete | Quién | Qué produce | Decisiones de las que sale | Lo revisa |
|---|---|---|---|---|
| **Alcance** | Maxi | Documento de alcance (roles, funcionalidades, reglas de negocio, estados, criterios de aceptación) y `README.md` | 1 y 2.1 a 2.7 | Male |
| **Arquitectura** | Carola | `docs/ARCHITECTURE.md`, diagramas de contexto y de contenedores, ADR D1 | 3.1 a 3.3 | Salvador |
| **Clima hacia afuera** | Male | Contrato OpenAPI, su documentación con ejemplos y errores, el mock, ADR D8 | 6.1 a 6.6 | Maxi |
| **Datos, comunicación y esqueleto** | Salvador | ADR D3, ADR D5 y las carpetas iniciales de los cuatro servicios | 4.1 a 4.4, 5.1, 5.2 y 7 | Carola |

Los pares de revisión son una sugerencia; el equipo todavía no los confirmó.

El detalle de qué tiene que hacer cada uno está en [entrega-1.md](entrega-1.md).

## Lo que estas decisiones dejan abierto

Elecciones que aparecen por lo que se decidió y que todavía nadie tomó:

1. **Caché:** Redis o Memcached (7.2).
2. **Hosting de Clima:** qué proveedor (7.2).
3. **API gateway:** con qué se hace; depende de la respuesta del profe (7.4).
4. **Aptitud de Clima:** una sola, o separada para aterrizar y para despegar (6.1).
5. **Fuente real del clima:** el documento sugería Open-Meteo, la que se usó en el práctico de la clase 7; falta confirmarla (6.2).
6. **Servicio Usuarios:** qué tipo de base usa y con qué patrón interno se organiza. No estaba en las decisiones 5.1 ni 7.3 porque el servicio surgió de la 3.3.
7. **Publicación del mock:** si se sube a internet para el viernes 9/10 (6.6).
8. **Valores:** duración del slot, timeouts, reintentos, vigencia de la caché, retraso tolerable del índice y umbrales de clima.
9. **Para la Entrega 2:** qué se cachea (D7), qué servicio se balancea (D12), qué mecanismo de resiliencia se implementa (D10), observabilidad (D11) y los bonus.

## Puntos a cuidar

Consecuencias de lo elegido que conviene tener presentes al diseñar:

- **La prioridad de los arribos (2.7) complica la operación crítica.** Asignar un slot a un arribo puede quitárselo a un despegue. Sacar un vuelo y poner otro tiene que ocurrir como una sola operación, aun con pedidos simultáneos, y generar el evento "slot cedido a un arribo" sin perderlo. Es el centro del ADR D4.
- **Pistas trabaja con el último clima conocido (4.2).** Si un aviso de Clima se demora, Pistas puede asignar un slot en una pista que debería estar cerrada. Hay que definir cuánto atraso se acepta.
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
| D8 | Contrato propio | 6.1 a 6.6 | Entrega 1 |
| D3 | Persistencia | 5.1, 5.2, 7.2 | Entrega 1, versión inicial |
| D5 | Comunicación entre servicios | 4.1 a 4.4 | Entrega 1, versión inicial |
| D2 | Arquitectura interna | 7.3 | Entrega 2 |
| D4 | Consistencia y concurrencia | 2.3, 2.7, 4.4 | Sin entrega asignada |
| D6 | Búsqueda | 3.2 | Entrega 2 |
