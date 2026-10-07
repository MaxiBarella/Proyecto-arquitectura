# Enunciado del Práctico Integrador 2026, analizado

Fuente: `Enunciado TP Final.pdf` (Arquitectura de Software 2026, Facultad de Ingeniería, UCC), 10 páginas. Analizado el 7/10/2026.

Este documento es la referencia de trabajo del equipo y de los agentes. Ante cualquier diferencia con [requisitos-del-profe.md](requisitos-del-profe.md) (notas de la clase del 2/10), manda el enunciado escrito; las diferencias detectadas están en la sección [Diferencias con lo dicho en clase](#diferencias-con-lo-dicho-en-clase).

Los identificadores (`2.6`, `D7`, etc.) son los del enunciado, para poder citarlos en ADRs, historias y PRs.

## En una página

- Hay que diseñar, construir y poner en marcha un sistema de **microservicios** sobre un dominio nuevo: testeado, observable, tolerante a fallas e **integrado con el sistema de otro grupo**.
- El enunciado fija **qué** cumplir, no **cómo**. Cada decisión de diseño, arquitectura y tecnología es nuestra y hay que documentarla, justificarla y defenderla.
- Mínimos duros: 3 microservicios + API gateway, frontend web, balanceo de carga sobre 2+ instancias, comunicación síncrona y asíncrona, motor de búsqueda, caché, almacenamiento relacional y no relacional, una operación con garantías de consistencia, un mecanismo de resiliencia, observabilidad, tests unitarios / integración / carga / contrato, y arranque local con un único comando.
- 13 decisiones obligatorias (D1 a D13), cada una en su ADR.
- Publicamos una capacidad con contrato formal versionado y desplegada en la nube; consumimos la de otro grupo guiándonos solo por su contrato.
- **Próximo vencimiento: viernes 9/10, Entrega 1 (diseño y contrato con mock).**

## Dominio (sección 1)

Reglas del enunciado:

- Distinto al del TP de Desarrollo de Software y al de los demás grupos. Lo aprueban los profesores.
- No puede ser solo ABM y consulta. La **acción principal** tiene que involucrar restricciones, estados, validaciones o consecuencias de negocio.
- Ejemplo del enunciado: turnos médicos, entidad profesional/turno, acción "reservar un turno", característica "dos personas no pueden tomar el mismo turno; picos de demanda".

Estado del grupo (7/10/2026): el dominio es **aeropuertos** y el recorte elegido es **arribos y despegues, con gestión de vuelos, asignación de pista y clima**. El clima es la capacidad que se publica para los otros grupos; el profe ya había sugerido este recorte. Queda excluido todo lo relacionado con ventas.

Decidido el 7/10/2026: son **tres microservicios, Clima, Vuelos y Pistas**, más el API gateway. Las responsabilidades de cada uno, la propiedad de los datos y el flujo entre ellos todavía no están decididos; cuando se cierren van al ADR de D1.

### Qué tiene que permitir el recorte que elijamos

El recorte condiciona si los requisitos salen naturales o forzados. Estas son las preguntas que hay que responder sobre el recorte elegido:

| Necesitamos | Para cumplir | Pregunta a responder |
|---|---|---|
| Una acción principal con conflicto por un recurso escaso | 1, 2.8, D4 | ¿Qué cosa no pueden tomar dos a la vez? ¿Qué no se puede duplicar ni perder? |
| Una entidad con estados y transiciones | 1 | ¿Qué ciclo de vida tiene y qué transiciones son inválidas? |
| Una entidad que valga la pena buscar con filtros y orden | 2.6, D6 | ¿Qué busca el usuario, por qué filtros y en qué orden? |
| Una lectura frecuente y costosa | 2.7, D7 | ¿Qué se lee mucho más de lo que cambia, y cuánta desactualización tolera? |
| Un hecho de negocio que le interese a otro servicio | 2.5, D5 | ¿Qué evento de dominio se publica y quién reacciona? |
| Al menos tres responsabilidades separables con datos propios | 2.1, D1 | ¿Dónde están los límites y quién es dueño de cada dato? |
| Datos de naturaleza distinta | 2.8, D3 | ¿Qué pide relacional y qué pide no relacional, y por qué? |
| Algo simple y útil para un tercero | 4, D8 | ¿Qué capacidad chica podemos publicar primero? |
| Un flujo importante donde entre un dato externo | 4, D9 | ¿En qué paso del flujo principal usamos al proveedor y qué pasa si no responde? |

## Requisitos de arquitectura y desarrollo (sección 2)

La numeración salta de 2.3 a 2.5: en el enunciado no existe 2.4.

### 2.1 Arquitectura de servicios

- Al menos **tres microservicios** y **un API gateway**.
- Al menos dos servicios con **patrones de arquitectura interna diferentes**, de los trabajados en la materia. En la presentación tienen que ser "reconocibles".
- El gateway es el punto de entrada único y debe resolver **autenticación, direccionamiento y control de tráfico** (lo exige la presentación grupal).

### 2.2 Frontend

- Una interfaz web que centraliza la interacción y cubre **todas** las funcionalidades.
- Para la presentación: completo, desde el ingreso del usuario hasta la funcionalidad compartida.

### 2.3 Balanceo de carga

- Sobre al menos un servicio desplegado en **dos o más instancias**.
- Tiene que **detectar cuándo una instancia no está disponible**.
- La distribución de solicitudes y el estado de las instancias se comprueban **con la observabilidad del sistema**, no a mano.

### 2.5 Comunicación síncrona y asíncrona

- Síncrona: definir **timeout**, manejo de errores, qué pasa con respuestas tardías o ausentes, y **política de reintentos**.
- Asíncrona: mensajería, con **publicación y consumo de un evento de dominio** y **tratamiento de mensajes fallidos**.
- Para la presentación: el consumidor tiene que ser **idempotente** y debe haber tratamiento de mensajes no procesables.

### 2.6 Búsqueda

- Una entidad del dominio con búsqueda implementada sobre un **motor de búsqueda**.
- Debe soportar **paginación**, **filtros relevantes para el dominio** y **al menos un criterio de ordenamiento**.
- El índice se mantiene **sincronizado con los cambios** del sistema.
- Hay que **definir el retraso máximo tolerable** entre una modificación en los datos fuente y su aparición en los resultados.
- Debe existir un mecanismo de actualización o reconstrucción del índice.

### 2.7 Caché

- En al menos un **flujo de lectura relevante**.
- La elección se basa en una **necesidad observable**, no en cumplir el requisito.
- **Evidencia con métricas** del impacto: medición comparativa antes y después.

### 2.8 Consistencia

- Cada servicio elige el almacenamiento según sus patrones de acceso, volumen, necesidad de consulta y garantías de consistencia.
- Al menos **dos tipos de almacenamiento**: uno relacional y uno no relacional.
- Al menos **una operación con garantías de consistencia**: no puede duplicarse ni perderse información.

### 2.9 Resiliencia

- Identificar las **dependencias de cada servicio** y definir qué pasa cuando cada una falla o responde lento.
- Implementar **al menos un mecanismo de protección** frente a fallas.
- Demostrarlo con una **caída provocada** y registrarlo en `docs/POSTMORTEM.md`.

### 2.10 Observabilidad

- Tiene que poder entenderse el estado de una operación sin inspeccionar cada componente a mano.
- Mínimo: **logs estructurados**, **métricas** y **un tablero** de estado general.
- Otras secciones suman: logs **correlacionados** y una **traza distribuida** del flujo principal (Entrega 2); **trazas completas**, **una alerta**, **objetivo de servicio** y criterios de alerta (D11 y defensa).

### 2.11 Testing

1. **Unitarios** sobre reglas de negocio relevantes, "superando el porcentaje que represente calidad". El enunciado no da el número: lo definimos y lo justificamos nosotros.
2. **Integración por servicio** contra una dependencia real (almacenamiento o mensajería).
3. **Carga**, con resultados documentados y analizados.
4. **Contrato** sobre la capacidad consumida, capaz de detectar cambios incompatibles (viene de la sección 4).

### 2.12 Puesta en marcha

- Código y documentación en **un único repositorio público**. La versión evaluable está en la **rama principal**.
- Definir y **usar** una estrategia de ramas, revisiones e integración.
- Arranque local con **un procedimiento único, automatizado y documentado**, que cree o inicie las dependencias y no dependa de configuraciones manuales de nuestras máquinas.
- **Secretos y datos sensibles fuera del repositorio.**

## Documentación (sección 3)

Se evalúa junto con el código y tiene que mantenerse al día durante el desarrollo, no armarse al final.

| Archivo | Contenido mínimo |
|---|---|
| `README.md` | Dominio, objetivo y flujo principal. Cómo ejecutar localmente, cómo acceder a la parte desplegada y dónde está el resto de la documentación. |
| `SPEC.md` o equivalente | Funcionalidades, reglas de negocio y criterios de aceptación. Vale el artefacto de la metodología elegida (en nuestro caso, lo que produce BMAD en `_bmad-output/`), siempre que cumpla esa finalidad. |
| `docs/ARCHITECTURE.md` | Arquitectura general con texto y diagramas: servicios, responsabilidades, datos de cada uno, comunicaciones y distribución de componentes. Para la presentación suma una sección de **limitaciones conocidas y deuda técnica aceptada**. |
| `docs/adr/ADR-XXX.md` | Un archivo por decisión. Las obligatorias (D1 a D13) y cualquier otra relevante. |
| `docs/contracts/README.md` | Capacidad publicada y contrato versionado: versiones, ejemplos de uso, errores y todo lo necesario para que otro grupo la consuma. |
| `docs/POSTMORTEM.md` | Informe de la caída provocada: impacto, línea temporal, detección, respuesta, causa y acciones de mejora. |

## Integración entre grupos (sección 4)

Los profesores asignan quién provee a quién.

### Como proveedores

- Publicar y documentar una capacidad de modo que el otro grupo se integre **sin explicaciones privadas**.
- La primera versión operativa tiene que estar en un **entorno accesible** y **seguir operativa hasta el final de la evaluación**. Para la presentación: implementación real, en la nube, con URL pública.
- **Contrato formal y versionado** en `docs/contracts/`, en un **formato estándar y procesable** adecuado al mecanismo (API, eventos o ambos).
- El contrato y su documentación indican:
  - operaciones o eventos disponibles;
  - qué se envía y qué se obtiene;
  - errores posibles y cómo interpretarlos;
  - condiciones de autenticación, idempotencia o uso;
  - versión vigente;
  - ejemplos suficientes para implementar y probar.

### Como consumidores

- La capacidad del proveedor entra en **un flujo importante** de nuestro sistema. No valen una llamada aislada, una pantalla de prueba ni una integración sin consecuencias de negocio.
- La llamada sale **del microservicio responsable** de esa funcionalidad. **No desde el frontend ni a través de nuestro gateway.**
- Se trabaja solo a partir del contrato y la documentación publicados.
- Hay que:
  - respetar el contrato publicado;
  - incluir un **test de contrato** que detecte cambios incompatibles;
  - definir timeouts y tratamiento de errores;
  - decidir **qué ve el usuario cuando el proveedor falla**;
  - demostrarlo en la jornada de fallas controladas.

## Decisiones arquitectónicas obligatorias (sección 5)

Cada ADR explica por qué se eligió una alternativa, qué otras se evaluaron y qué consecuencias se aceptaron. Se registran durante el diseño y la implementación.

Si una decisión cambia, **no se borra**: el ADR anterior se marca como reemplazado y el nuevo indica a cuál reemplaza y por qué.

| # | Decisión | Qué tiene que quedar justificado | Vence |
|---|---|---|---|
| D1 | Límites de los servicios | Criterios de separación, relaciones entre servicios, propiedad de los datos | E1; se valida en E2 |
| D2 | Arquitectura interna | Estilos o patrones, dependencias internas, adecuación al problema de cada servicio | E2 |
| D3 | Persistencia | Almacenamiento por servicio, patrones de acceso, limitaciones aceptadas | E1 (inicial); se valida en E2 |
| D4 | Consistencia y concurrencia | Operaciones con garantías especiales; concurrencia, duplicación, pérdida, fallas parciales | Sin entrega asignada (ver ambigüedades) |
| D5 | Comunicación entre servicios | Síncrono y asíncrono, timeouts, reintentos, eventos, idempotencia | E1 (inicial); se valida en E2 |
| D6 | Búsqueda | Qué se indexa, criterios de consulta, actualización o reconstrucción, retraso aceptable | E2 |
| D7 | Caché | Qué se guarda, invalidación, vigencia, comportamiento ante fallas, criterios de medición | E2 |
| D8 | Contrato propio | Diseño, publicación, compatibilidad, estrategia de versionado | E1; se actualiza en E2 si cambió |
| D9 | Consumo del proveedor | Incorporación, adaptación al contrato, comportamiento ante errores, cambios o indisponibilidad | E2 |
| D10 | Resiliencia | Comportamiento ante fallas, mecanismos para limitar la propagación o degradar de forma controlada | E2 |
| D11 | Observabilidad | Logs, métricas, trazas, tableros, objetivo de servicio, criterios de alerta | E2 (primera versión) |
| D12 | Balanceo de carga | Servicio elegido, estrategia de distribución, verificación de disponibilidad, caída de una instancia | E2 |
| D13 | Capacidad y costos | Resultados de carga, principal límite, alternativa de escalado, estimación de costos | E2 |

## Cronograma y entregables (sección 6)

### Viernes 2/10 — Instancia inicial (cumplida)

- Integrantes confirmados.
- Dominio y alcance preliminar aprobados.
- Repositorio creado y accesible para la cátedra.

### Viernes 9/10 — Entrega 1: diseño y contrato con mock

- [ ] Primera versión de `README.md`
- [ ] Primera versión de la documentación de alcance (SPEC o equivalente BMAD)
- [ ] Primera versión de `docs/ARCHITECTURE.md`
- [ ] Diagrama de contexto
- [ ] Diagrama de contenedores
- [ ] Límites preliminares de los servicios, responsabilidades y propiedad de los datos
- [ ] Capacidad propia a exponer seleccionada y documentada (contrato en `docs/contracts/`, con mock según el título de la entrega)
- [ ] Estructura inicial de los servicios y sus dependencias
- [ ] ADR D1 y D8
- [ ] ADR D3 y D5 en versión inicial

### Viernes 23/10 — Entrega 2: dominio propio de punta a punta

- [ ] Al menos un servicio operativo
- [ ] La funcionalidad a compartir funcionando
- [ ] Al menos un tipo de almacenamiento integrado al flujo
- [ ] Logs estructurados y correlacionados
- [ ] Primera traza distribuida del flujo principal
- [ ] ADR D2, D6, D7, D9, D10, D12 y D13
- [ ] Validación de D1, D3 y D5
- [ ] Primera versión de D11
- [ ] Actualización de D8 si el contrato cambió
- [ ] Anuncio de los bonus elegidos (sección 7)

### Miércoles 11/11 o viernes 13/11 — Presentación grupal

La fecha de cada grupo se sortea y se comunica el 6/11.

- [ ] Frontend completo, del ingreso del usuario a la funcionalidad compartida
- [ ] Punto de entrada único con autenticación, direccionamiento y control de tráfico
- [ ] Al menos tres servicios operativos
- [ ] Capacidad propia real, desplegada en la nube, con URL pública
- [ ] Capacidad del proveedor incorporada al flujo principal
- [ ] Al menos dos tipos de almacenamiento integrados al flujo
- [ ] Caché en un flujo de lectura relevante, con medición antes y después
- [ ] Dos estilos o patrones de arquitectura interna reconocibles
- [ ] Evento de dominio publicado y consumido, consumidor idempotente, tratamiento de mensajes no procesables
- [ ] Búsqueda indexada con paginación, filtros, ordenamiento y actualización o reconstrucción del índice
- [ ] Verificación de las decisiones con evidencia obtenida del sistema
- [ ] Limitaciones conocidas y deuda técnica aceptada en `docs/ARCHITECTURE.md`

### Defensa individual (examen final)

- [ ] Operación crítica con las garantías de consistencia y concurrencia definidas
- [ ] Tests unitarios y un test de integración contra una dependencia real
- [ ] Test de contrato sobre la capacidad consumida
- [ ] Balanceo operativo sobre 2+ instancias, con verificación de disponibilidad y evidencia de la distribución
- [ ] Simulación de la caída de una dependencia propia, del proveedor externo o de ambas
- [ ] Test de carga ejecutado: resultados, análisis de capacidad, principal límite
- [ ] Comportamiento documentado ante la caída de cada dependencia relevante, y al menos un mecanismo de protección
- [ ] Logs, métricas y trazas completos, con el tablero y la alerta que se usan en la presentación
- [ ] Evidencia de una caída controlada hecha como ensayo y su informe en `docs/POSTMORTEM.md`

La defensa es individual: cada integrante tiene que poder explicar y justificar cualquier decisión del sistema.

## Bonus (sección 7)

Hasta **dos** desafíos, hasta **un punto** cada uno. Se **anuncian en la Entrega 2** y se incorporan al alcance documentado. Para sumar, el desafío tiene que estar integrado, documentado, testeado y defendido.

Alternativas que menciona el enunciado:

- un agente que ejecute operaciones del sistema mediante un protocolo estándar;
- actualizaciones de información en tiempo real;
- una capa de agregación adaptada a las necesidades de la interfaz;
- orquestación de contenedores;
- tests e2e;
- escalado automático.

## Ambigüedades e inconsistencias del enunciado

Puntos para consultar con la cátedra o resolver con una decisión propia documentada:

1. **Formato de ADR.** La sección 3 remite al "formato definido en la sección 8", pero el enunciado termina en la sección 7. Consultar; mientras tanto, usar un formato que cubra lo que pide la sección 5: contexto, alternativas evaluadas, decisión, consecuencias aceptadas, estado y a qué ADR reemplaza.
2. **D4 no tiene entrega asignada.** No figura en la Entrega 1 ni en la 2, pero la operación crítica se evalúa en la defensa. Conviene escribirlo junto con D3 y D5.
3. **Falta la sección 2.4.**
4. **La sección 2.10 termina en dos puntos** y no lista nada después. Lo exigible sale de sumar 2.10, la Entrega 2, D11 y la defensa.
5. **Trazas.** No están en el mínimo de 2.10, pero sí en la Entrega 2, en D11 y en la defensa. Tratarlas como obligatorias.
6. **Mock en la Entrega 1.** Aparece en el título ("contrato con mock") y no en la lista de entregables. Asumir que hay que entregar el contrato con un mock utilizable por el grupo consumidor.
7. **Cobertura de tests unitarios.** "El porcentaje que represente calidad" no está definido. Fijar un umbral propio y justificarlo.
8. **D12 y D13 vencen en la Entrega 2**, antes de tener balanceo operativo y pruebas de carga ejecutadas (que se piden en la defensa). Se entregan como decisión y plan, y se completan con la evidencia.
9. **"Jornada de fallas controladas".** Se menciona como una instancia propia, sin fecha en el cronograma.
10. **Entrega 2 pide "al menos un servicio operativos"**; para la presentación son tres.

## Diferencias con lo dicho en clase

Respecto de [requisitos-del-profe.md](requisitos-del-profe.md):

| Tema | En clase | En el enunciado |
|---|---|---|
| 9/10 | "Tres microservicios levantados y comunicándose aunque no hagan nada" | "Estructura inicial de los servicios y sus dependencias". No exige que estén corriendo. |
| 16/10 | No se entrega nada | No figura. Coincide. |
| 23/10 | "Entrega con presentación grupal" | Es la Entrega 2. La presentación grupal es el 11 o el 13 de noviembre. |
| Hosting | Gratuito y simple; nada de AWS ni Azure | Solo dice "en la nube" con URL pública. |
| Picos de carga | De 10 a 1000 pedidos por segundo | No da números. |
| Documentar el uso de IA | Obligatorio | No se menciona. |
| Caché propia desaprueba | Sí | No se menciona. |
| Gateway | No se mencionó | Obligatorio, con autenticación, direccionamiento y control de tráfico. |
| Test de contrato | No se mencionó | Obligatorio. |
| Trazas, alerta, objetivo de servicio | No se mencionaron | Exigidos en D11 y en la defensa. |

Lo que solo se dijo en clase conviene seguir respetándolo: no contradice al enunciado.
