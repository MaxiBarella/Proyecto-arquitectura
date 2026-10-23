# Entrega 2: qué le toca a cada pareja

**Vence el viernes 23/10/2026.** Se llama "Dominio propio de punta a punta". Es la primera entrega que pide código funcionando.

Este documento responde a la pregunta "¿qué tengo que hacer?". Buscá tu nombre en la tabla y andá a la sección de tu pareja.

| Pareja | Integrantes | Paquete | Revisa sus pull requests |
|---|---|---|---|
| 1 | Carola (`JalilCarola`) y Maxi (`MaxiBarella`) | [Vuelos y Pistas](#pareja-1--vuelos-y-pistas-carola-y-maxi) | Alguien de la pareja 2 |
| 2 | Male (`malenagriffi`) y Salvador (`salvadorsolana04`) | [Clima y lo transversal](#pareja-2--clima-y-lo-transversal-male-y-salvador) | Alguien de la pareja 1 |

El equipo eligió trabajar de a dos (decisión 19) y Maxi informó las parejas. Qué hace cada pareja lo propuso el agente de IA, siguiendo la opción C del documento de decisiones ("dos con Clima y lo transversal, dos con Vuelos y Pistas") y lo que hizo cada uno en la Entrega 1. Queda confirmado cuando el equipo apruebe el pull request.

## Qué pide la entrega

**Código**

- Al menos un servicio operativo.
- La funcionalidad a compartir funcionando: Clima real, ya no el mock.
- Al menos un tipo de almacenamiento integrado al flujo.
- Logs estructurados y correlacionados.
- Primera traza distribuida del flujo principal.

**Documentación**

- ADR nuevos: D2, D6, D7, D9, D10, D12 y D13.
- Primera versión del ADR D11.
- Validación de D1, D3 y D5 contra el sistema funcionando.
- Actualización de D8 si el contrato cambió.
- Anuncio de los bonus: no tomamos ninguno (17.1).

## Para todos

1. **Leé primero** las secciones 8 a 20 de [decisiones.md](decisiones.md). Lo que hay que construir sale de ahí.
2. **Trabajá en una rama propia** y abrí un pull request hacia `main`, como dice [reglas-de-versionado.md](reglas-de-versionado.md). **Lo revisa alguien de la otra pareja**, así cada uno conoce las dos mitades del sistema para la defensa.
3. **Cada pareja toca sus carpetas.** Lo compartido (`docker-compose.yml` y `.env.example`) lo edita la pareja 2. Si la pareja 1 necesita algo ahí, lo pide o abre un pull request chico avisando.
4. **Si aparece algo sin decidir, frená y llevalo al grupo.** Más abajo está lo que ya se sabe que falta.
5. **Los ADR se escriben con la plantilla.** Copiá [adr/PLANTILLA.md](adr/PLANTILLA.md) como `ADR-XXX.md` dentro de `docs/adr/`, sin cambiar los títulos ni el orden de las secciones. El número es el de la decisión del enunciado (D2 es `ADR-002`), como explica [adr/README.md](adr/README.md).

### Lo que comparten las dos parejas

- **Identidad del usuario (15.6).** Vuelos y Pistas leen quién hace el pedido de los encabezados `X-User-Id`, `X-User-Role` y `X-Airline`. El gateway llega en noviembre; hasta entonces, en las pruebas se mandan a mano.
- **Observabilidad (11).** La pareja 2 arma primero Grafana en el Docker Compose y deja Clima con OpenTelemetry y logs en JSON. La pareja 1 copia esa misma configuración en Vuelos y Pistas. Cada servicio es un módulo de Go aparte, así que se copia, no se importa.
- **Librerías (20.1).** Las mismas en todos los servicios.
- **Eventos.** El formato está en el [ADR-005](adr/ADR-005.md). El identificador de la traza va en el campo de correlación del mensaje (11.2).

---

## Pareja 1 — Vuelos y Pistas: Carola y Maxi

**Qué es.** El flujo principal: una aerolínea publica un vuelo y le pide un slot a Pistas. Incluye la operación crítica, que son los pedidos simultáneos y la prioridad de los arribos (8.1).

**Código**

- **Pistas**
  - dos pistas cargadas como datos (2.1), con bloques de 15 minutos;
  - asignar un slot exacto y, si está ocupado, rechazar el pedido e informar los 3 libres más cercanos (18);
  - prioridad de los arribos: un arribo se queda con el slot de un despegue, y se publica "slot cedido a un arribo" (2.7);
  - dos pedidos simultáneos por el mismo slot: lo obtiene uno solo, con la restricción única de la base (4.6);
  - outbox y relay para "slot asignado" y "slot cedido a un arribo" (4.4, 4.7).
- **Vuelos**
  - publicar un vuelo, que queda "pendiente de slot", con número de vuelo y ámbito (18), guardado en MySQL;
  - pedirle el slot a Pistas con timeout de 2 segundos, hasta 2 reintentos (4.5) y el circuit breaker (13.1);
  - consumir "slot asignado" (pasa a "programado") y "slot cedido a un arribo" (pasa a "demorado"), sin aplicar dos veces el mismo mensaje (4.9);
  - cada aerolínea opera solo sus vuelos (18).
- **Tests:** unitarios de las reglas de Pistas (asignación, rechazo con los más cercanos y prioridad). Si llegan, un test de integración contra MySQL real que pruebe que dos pedidos simultáneos no ocupan el mismo slot: es la mejor evidencia para la defensa.
- **Observabilidad:** logs en JSON y la traza del flujo, de Vuelos a Pistas, al evento y de vuelta a Vuelos.

**Documentación**

- **`SPEC.md` versión 0.3** (Maxi): cerrar los pendientes resueltos en la sección 18 de decisiones.md, sumar los datos del vuelo y los filtros del tablero (14.1), y anotar que no se toma ningún bonus (17.1).
- **ADR-002, arquitectura interna** (Carola): Pistas hexagonal, Vuelos CQRS, Clima y Usuarios en capas (7.3, 7.5). Para Clima, mirar el código de la pareja 2.
- **ADR-006, búsqueda:** qué se indexa, filtros, orden, paginación, atraso de 60 segundos y comando de reconstrucción (14). La búsqueda funcionando se muestra en noviembre.
- **ADR-010, resiliencia:** de qué depende cada servicio, qué pasa cuando falla cada dependencia y el circuit breaker de Vuelos a Pistas (13.1).
- **Validación del ADR-001 y del ADR-005** contra el sistema funcionando: qué se confirmó y qué cambió. En el ADR-005 hay que sumar el evento "Clima sin datos".
- **`docs/ARCHITECTURE.md`** (Carola): sumar el gateway (15), las dos copias de Clima (9.3) y la entrada directa del consumidor (9.9).

**De dónde sale.** Decisiones 2, 4, 8.1, 12.3, 13.1, 14, 15.6, 18 y 20.

**Quién hace qué.** Mitad y mitad: cada uno toma un servicio entero y la documentación más ligada a él. Lo propuso Carola el 16/10/2026; queda confirmado cuando Maxi apruebe el pull request.

| | Carola | Maxi |
|---|---|---|
| Código | Pistas | Vuelos |
| Tests | Unitarios de las reglas de Pistas y el de integración de pedidos simultáneos | Unitarios de las transiciones de estado de Vuelos |
| Endpoints (lunes 19/10) | Los de Pistas (la ruta del slot y las claves de ruteo las eligió Maxi, 4.14 y 4.15) | Los de Vuelos |
| Documentación | ADR-002, `ARCHITECTURE.md`, ADR-010 y validación del ADR-001 | `SPEC.md` 0.3, ADR-006 y validación del ADR-005 |

Los dos: la traza del flujo completo, de Vuelos a Pistas y de vuelta.

**Qué falta decidir**

- Nada que frene esta entrega. Los endpoints quedaron en las decisiones 4.14 a 4.16.

**Ojo con**

- La prioridad de los arribos y los pedidos simultáneos son lo más difícil y lo más preguntado en la defensa. Conviene arrancar por ahí.
- Vuelos va a correr en dos instancias (12.1): no se puede guardar nada importante en la memoria de una sola.
- El circuit breaker vive en cada instancia de Vuelos, así que cada una decide por su cuenta. Está bien, pero hay que saber explicarlo.

---

## Pareja 2 — Clima y lo transversal: Male y Salvador

**Qué es.** Clima real y publicado en la nube, y lo que usan todos los servicios: el Docker Compose, la observabilidad, y los ADR de caché, balanceo y capacidad.

**Código**

- **Clima**
  - consultar a Open-Meteo cada 15 minutos, guardar la observación en MongoDB y calcular la aptitud con los umbrales de la 6.11 (9.1, 9.2);
  - responder `GET /airports/SACO/conditions` con el último dato, y `503` si tiene más de 30 minutos (9.2, 6.19);
  - claves desde una variable de entorno (9.5) y límite de 60 pedidos por minuto por clave, en memoria (9.6);
  - publicar "aptitud operativa cambió" y "Clima sin datos" con el outbox dentro del documento de aptitud (4.7, 18);
  - la ruta interna para forzar una condición, que dura hasta que el operador la quita (6.2, 18);
  - una variable de entorno para que la copia de la nube no use RabbitMQ (9.3).
- **Publicación:** Clima en Render con MongoDB Atlas (9.4, 9.7). Sumar a `docs/contracts/README.md` la dirección pública y el aviso de que el servicio se duerme.
- **Docker Compose:** sumar Grafana (`grafana/otel-lgtm`, 11.1) y dejar Clima con OpenTelemetry y logs en JSON, como modelo para la pareja 1.
- **Tests:** unitarios del cálculo de aptitud y de cuándo un dato es viejo.

**Documentación**

- **ADR-007, caché:** tablero en Memcached, 60 segundos, sin borrar por evento, y qué pasa si Memcached se cae (10). Cómo se va a medir el antes y el después. La caché funcionando se muestra en noviembre.
- **ADR-008** (Male): sumar la dirección pública, el límite en Clima (9.6), el `503` por dato viejo (9.2) y el servicio dormido (9.4).
- **ADR-009, consumo del proveedor:** depende de la capacidad que asigne la cátedra. Escribir lo que ya se sabe (la llamada sale del servicio dueño de la funcionalidad, con timeout y test de contrato) y dejar el resto en "Pendiente".
- **ADR-011, observabilidad, primera versión:** 11.1 a 11.5, el objetivo de servicio y la alerta de mensajes fallidos.
- **ADR-012, balanceo:** 12.1 a 12.3 y los dos archivos de NGINX (15.3). El balanceo funcionando se muestra en noviembre.
- **ADR-013, capacidad y costos:** el plan de pruebas con k6 (la rampa de 10 a 1000 pedidos por segundo), qué se mide, cuál se espera que sea el límite y una estimación de costos. Se completa con los resultados.
- **Validación del ADR-003** (Salvador) contra el sistema funcionando.

**De dónde sale.** Decisiones 6, 9, 10, 11, 12, 15.3, 16 y 20.

**Quién hace qué.** Male y Salvador lo acordaron el 18/10/2026. La idea es que cada uno siga con lo que ya conoce de la Entrega 1: Male, que armó el contrato, se encarga de Clima y de lo que sale hacia afuera; Salvador, que armó el esqueleto de los servicios, se encarga de lo transversal.

| | Male | Salvador |
|---|---|---|
| Código | Clima: consulta a Open-Meteo, MongoDB, aptitud, `503` por dato viejo, claves, límite de pedidos, eventos y ruta para forzar | Grafana en el Docker Compose, y OpenTelemetry y logs en JSON en Clima |
| Tests | Unitarios del cálculo de aptitud y de cuándo un dato es viejo | |
| Publicación | Clima en Render con MongoDB Atlas, y la dirección en `docs/contracts/README.md` | |
| Documentación | ADR-008, ADR-009 y ADR-007 | ADR-011, ADR-012, ADR-013 y validación del ADR-003 |
| Orden | Programa Clima sobre la observabilidad de Salvador; mientras tanto, los ADR y la lógica de aptitud | Primero: la observabilidad, mergeada para el lunes 19/10 |

**Qué falta decidir**

- Nada que frene esta entrega. El ADR-009 queda incompleto hasta que la cátedra asigne el proveedor.

**Ojo con**

- La pareja 1 necesita la configuración de observabilidad para armar su traza: conviene mergearla primero, a principios de la semana.
- Render duerme el servicio, y con él se frena la consulta periódica. Hay que avisarlo en la guía del contrato.
- Las dos copias de Clima pueden mostrar datos distintos durante unos minutos. Hay que poder explicar por qué no rompe nada (9.3).

---

## Lo que queda para noviembre

- El gateway con NGINX (15) y su ADR-014.
- Usuarios: inicio de sesión y tokens (15.2, 15.5).
- El balanceo de Vuelos (12), la caché del tablero (10) con la medición de antes y después, y la búsqueda con Solr (14), funcionando.
- El cierre de pistas, las demoras por clima y los desvíos (8.1).
- El frontend.
- La capacidad del proveedor (D9), cuando la asigne la cátedra.

## Orden sugerido

Es una sugerencia para que una pareja no trabe a la otra.

| Cuándo | Qué |
|---|---|
| Lunes 19/10 | Pareja 2: Grafana en el Docker Compose y Clima con observabilidad, mergeado. Pareja 1: `SPEC.md` 0.3 y los endpoints propuestos. |
| Miércoles 21/10 | Los pull requests de código, abiertos. |
| Jueves 22/10 | Revisiones cruzadas, merge y los ADR abiertos. |
| Viernes 23/10 | Todo en `main` y el tag `entrega-2`. |

## Cuándo está terminada la entrega

- [x] Clima, Vuelos y Pistas operativos
- [x] Clima real funcionando y publicado en la nube
- [x] Almacenamiento integrado al flujo: MongoDB en Clima y MySQL en Vuelos y Pistas
- [x] Logs estructurados y correlacionados
- [x] Primera traza distribuida del flujo principal
- [x] ADR-002, ADR-006, ADR-007, ADR-009, ADR-010, ADR-012 y ADR-013
- [x] ADR-011, primera versión
- [x] ADR-001, ADR-003 y ADR-005 validados
- [x] ADR-008 actualizado: lo reemplaza el ADR-015, por el cambio de versionado (9.10)
- [x] `SPEC.md` 0.3, con el anuncio de que no hay bonus (hoy va por la 0.4)
- [x] Todo mergeado en `main` y el tag `entrega-2`
