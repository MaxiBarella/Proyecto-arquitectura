# Entrega 1: qué le toca a cada uno

**Vence el viernes 9/10/2026.** Se llama "Diseño y contrato con mock". No se pide código funcionando.

Este documento responde a la pregunta "¿qué tengo que hacer?". Buscá tu nombre en la tabla y andá a tu sección.

| Integrante | Usuario de GitHub | Paquete | Lo revisa |
|---|---|---|---|
| Maxi | `MaxiBarella` | [Alcance](#alcance--maxi) | Male |
| Carola | `JalilCarola` | [Arquitectura](#arquitectura--carola) | Salvador |
| Male | `malenagriffi` | [Clima hacia afuera](#clima-hacia-afuera--male) | Maxi |
| Salvador | `salvadorsolana04` | [Datos, comunicación y esqueleto](#datos-comunicación-y-esqueleto--salvador) | Carola |

Los pares de revisión son una propuesta que el equipo todavía no confirmó.

## Para todos

1. **Leé primero** [decisiones.md](decisiones.md): tu paquete consiste en poner por escrito lo que el equipo ya decidió ahí. No se decide nada nuevo por cuenta propia.
2. **Trabajá en una rama propia** y abrí un pull request hacia `main`, como dice [reglas-de-versionado.md](reglas-de-versionado.md).
3. **No toques los archivos de otro paquete.** Cada uno tiene los suyos, justamente para no pisarse.
4. **Si aparece algo sin decidir, frená y llevalo al grupo.** Más abajo está la lista de lo que ya se sabe que falta.
5. **Formato de los ADR** (decidido): título, fecha, estado, contexto, alternativas evaluadas, decisión, consecuencias aceptadas y a qué ADR reemplaza. Cada ADR es un archivo en `docs/adr/`.

### Dos cosas a acordar entre los cuatro antes de crear archivos

- **Cómo se numeran los ADR.** Tres personas van a crear ADR al mismo tiempo, así que hay que evitar que dos usen el mismo número. Propuesta sin confirmar: usar el número de la decisión del enunciado (`ADR-001` para D1, `ADR-003` para D3, `ADR-005` para D5, `ADR-008` para D8).
- **Con qué se hacen los diagramas.** Tienen que poder verse desde el repo.

---

## Alcance — Maxi

**Qué es.** Poner por escrito qué hace el sistema: quién lo usa, qué puede hacer y con qué reglas.

**Qué producís**

- El documento de alcance, con:
  - los roles (aerolínea, operador y visitante del tablero) y qué puede hacer cada uno;
  - las funcionalidades;
  - las reglas de negocio: slots, asignación, prioridad de los arribos, cierre de pista, demoras y desvíos;
  - los estados de un vuelo y las transiciones válidas e inválidas;
  - criterios de aceptación para cada regla.
- `README.md`: dominio, objetivo, flujo principal, cómo ejecutar el sistema y dónde está el resto de la documentación.

**De dónde sale.** Decisiones 1 y 2.1 a 2.7.

**Dónde va.** El alcance es el `SPEC.md` de la raíz (decidido el 8/10).

**Qué falta decidir**

- Los catorce puntos de la sección "Pendiente de definir" del `SPEC.md`.

**Ojo con**

- La prioridad de los arribos (2.7) es la regla más delicada: describí con un ejemplo qué pasa cuando un arribo pide un slot que tiene un despegue.
- El README de hoy describe la instalación de BMAD, no el proyecto. Hay que reescribirlo sin perder la sección de forma de trabajo.

---

## Arquitectura — Carola

**Qué es.** Explicar con texto y dibujos cómo está armado el sistema y por qué son esos servicios.

**Qué producís**

- `docs/ARCHITECTURE.md`, con los servicios, sus responsabilidades, los datos de cada uno, cómo se comunican y cómo se distribuyen.
- **Diagrama de contexto:** el sistema como una caja, y alrededor quiénes lo usan (aerolínea, operador, visitante) y con qué sistemas externos habla (el grupo que consume Clima, el grupo proveedor, la fuente del clima).
- **Diagrama de contenedores:** el frontend, el gateway, los cuatro servicios (Vuelos, Pistas, Clima, Usuarios), sus bases de datos, el motor de búsqueda y la mensajería.
- **ADR D1, límites de los servicios:** con qué criterio se separaron, cómo se relacionan y quién es dueño de cada dato.

**De dónde sale.** Decisiones 3.1 a 3.3. Para dibujar las flechas y las bases, también las 4.1, 4.2, 5.1, 5.2 y 7.2.

**Qué falta decidir**

- Con qué se hace el API gateway: está pendiente de una consulta al profe. En el diagrama alcanza con una caja "API gateway".
- Qué base usa el servicio Usuarios.

**Ojo con**

- Los diagramas y el ADR D1 tienen que decir lo mismo que los ADR de Salvador (bases y comunicación). Por eso se propone que se revisen entre ustedes.
- Todavía no se sabe qué capacidad nos toca consumir de otro grupo: en el diagrama de contexto va como "proveedor externo, a asignar".

---

## Clima hacia afuera — Male

**Qué es.** Definir y documentar lo que le ofrecemos al grupo que nos toque como consumidor, de modo que pueda integrarse sin preguntarnos nada.

**Qué producís**

- **El contrato**, en formato OpenAPI, en `docs/contracts/`.
- `docs/contracts/README.md`: qué operaciones hay, qué se envía y qué se recibe, qué errores existen y cómo interpretarlos, cómo se autentica, qué versión está vigente y ejemplos suficientes para probar.
- **El mock**, generado a partir del archivo del contrato.
- **ADR D8, contrato propio:** diseño, publicación, compatibilidad y estrategia de versionado.

**De dónde sale.** Decisiones 6.1 a 6.6:

- ofrece condiciones actuales por aeropuerto y aptitud operativa (6.1);
- datos reales, con posibilidad de forzar una condición (6.2);
- API descrita con OpenAPI (6.3);
- una clave por consumidor (6.4);
- la versión va en la dirección, `/v1/...` (6.5);
- mock generado del contrato (6.6).

**Qué falta decidir**

- Si la aptitud es una sola o separada para aterrizar y para despegar. Cambia la forma de la respuesta.
- Si la fuente real es Open-Meteo, la del práctico de la clase 7. Define qué datos se pueden devolver.
- Los umbrales de aptitud: con qué viento o visibilidad se considera que no se puede operar.
- Si el mock se publica en internet para el viernes, y en qué hosting.
- Con qué herramienta se genera el mock.

**Ojo con**

- El contrato tiene que distinguir los errores que tiene sentido reintentar de los que no (lo pide la clase de resiliencia).
- Lo que no esté escrito, para el otro grupo no existe. Probá leer tu documentación como si no supieras nada del proyecto.
- Forzar una condición es una función del operador, interna: no forma parte de lo que se publica.

---

## Datos, comunicación y esqueleto — Salvador

**Qué es.** Dejar escrito dónde guarda cada servicio sus datos y cómo se hablan entre ellos, y armar las carpetas vacías de los servicios.

**Qué producís**

- **ADR D3, persistencia (versión inicial):** qué almacenamiento usa cada servicio, qué accesos tiene que soportar y qué limitaciones se aceptan.
- **ADR D5, comunicación (versión inicial):** qué va por llamada directa y qué por eventos, con timeouts, reintentos, eventos e idempotencia.
- **La estructura inicial** de los cuatro servicios y sus dependencias, sin lógica.

**De dónde sale.** Decisiones 4.1 a 4.4, 5.1, 5.2 y 7:

- Vuelos y Pistas en MySQL, Clima en MongoDB, una instancia por servicio (5.1, 5.2);
- asignación de slot en dos pasos, llamada más evento (4.1);
- Clima avisa los cambios de aptitud por evento (4.2);
- la lista de eventos (4.3);
- Transactional Outbox con relay por sondeo (4.4);
- Go con Gin, RabbitMQ, Solr, Docker Compose (7.1, 7.2);
- Pistas Hexagonal, Vuelos CQRS, Clima en capas (7.3).

**Qué falta decidir**

- Qué base usa el servicio Usuarios y con qué patrón se organiza.
- La estructura de carpetas: cómo se llama cada carpeta y cómo se ordena por dentro cada servicio según su patrón.
- Los valores concretos de timeouts y reintentos. En esta versión inicial pueden quedar como "a definir".
- La caché (Redis o Memcached) no entra en esta entrega.

**Ojo con**

- Los tres patrones internos son distintos, así que las carpetas de Pistas, Vuelos y Clima no van a ser iguales entre sí.
- El enunciado pide que la llamada al proveedor externo salga del servicio que la usa, no del gateway ni del frontend. Conviene dejarlo previsto en el ADR D5.
- Con una instancia de base por servicio, más Solr y RabbitMQ, son muchos contenedores: vale la pena probar pronto que levanten en las computadoras de todos.

---

## Cuándo está terminada la entrega

- [ ] `README.md`, primera versión
- [ ] Documento de alcance, primera versión
- [ ] `docs/ARCHITECTURE.md`, primera versión
- [ ] Diagrama de contexto
- [ ] Diagrama de contenedores
- [ ] Límites de los servicios, responsabilidades y propiedad de los datos
- [ ] Capacidad propia documentada, con contrato y mock
- [ ] Estructura inicial de los servicios y sus dependencias
- [ ] ADR D1 y D8
- [ ] ADR D3 y D5, versión inicial
- [ ] Todo mergeado en `main` y el tag `entrega-1`
