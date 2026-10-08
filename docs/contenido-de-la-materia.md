# Contenido de la materia

Resumen de lo que se vio en Arquitectura de Software 2026 (UCC), para que las opciones que se propongan en el proyecto salgan de lo trabajado en clase. El enunciado exige usar patrones "trabajados durante la materia".

Fuentes, relevadas el 7/10/2026:

- Los apuntes teóricos de las clases, en PDF.
- El repositorio de ejercicios del profe: <https://github.com/TomiCassanelli/arquitectura-de-software>

Esto describe lo que enseñó la materia. No es una decisión del equipo: las herramientas y patrones del proyecto se eligen según [AGENTS.md](../AGENTS.md).

## Herramientas usadas en los prácticos

| Pieza | En los prácticos | Solo en el teórico |
|---|---|---|
| Lenguaje y framework | Go con Gin | — |
| Organización del código | Capas: `controllers`, `services`, `repositories`, `models` | Hexagonal, Clean, CQRS, Event Sourcing |
| Base no relacional | MongoDB (clase 2) | — |
| Base relacional | — | PostgreSQL en los ejemplos; MySQL con GORM como referencia al proyecto de Desarrollo de Software |
| Caché | Memcached y caché en memoria con `ccache` (clase 3) | Redis |
| Mensajería | RabbitMQ con `amqp091-go` (clase 4 y TP 1-4) | Kafka, como otra familia |
| Motor de búsqueda | Solr 9 (clase 5) | Elasticsearch, OpenSearch |
| Balanceador | NGINX con tres réplicas (clase 7) | — |
| API gateway | — | Conceptos; sin herramienta concreta |
| Infraestructura local | Docker Compose | — |
| Frontend | — | React, como cliente en los diagramas |
| Observabilidad | — | Todavía no se dio |

El práctico de la clase 7 es una API de clima en Go que consulta Open-Meteo, con tres réplicas detrás de NGINX.

## Temas por clase

### Clase 1 — Monolito y microservicios

- Un microservicio se delimita por capacidad de negocio, es dueño de sus datos y se comunica por contratos explícitos.
- El aislamiento ante fallos no es automático: hay que diseñarlo con límites, timeouts y degradaciones.
- Condiciones mínimas para distribuir: despliegue automatizado, observabilidad centralizada, gestión de secretos y contratos versionados.
- Expand and Contract: agregar es compatible; quitar, renombrar o cambiar un tipo rompe a los consumidores.

### Clase 2 — Datos en microservicios

- **Database per Service:** cada servicio es el único dueño de sus datos. Tres niveles de aislamiento: tabla separada, esquema separado, instancia separada. La base compartida es un antipatrón.
- Entre servicios no hay claves foráneas: se guardan identificadores y la validación es del servicio dueño.
- **Saga:** secuencia de transacciones locales con compensaciones, por coreografía o por orquestación.
- **Dual write** y **Transactional Outbox:** el cambio de negocio y el evento pendiente se guardan en la misma transacción; un relay publica después. La entrega es al menos una vez.
- **Idempotencia** del consumidor: registrar el identificador del mensaje junto con el efecto, en la misma transacción.
- Si varias operaciones tienen que ser atómicas, es señal de que pertenecen al mismo servicio.
- Criterios para elegir base: estructura del dato, tipo de consultas, patrón de acceso, escala, consistencia requerida y madurez del equipo. "NoSQL por moda" es el error más común.
- MongoDB: embeber o referenciar según cómo se lee; riesgos de esquema sin diseño, índices faltantes y duplicación.

### Clase 3 — Caché

- La caché es una copia temporal, nunca la fuente de verdad. Las operaciones críticas validan contra la fuente.
- Buenos candidatos: datos muy leídos, costosos de obtener, que toleran una ventana de desactualización.
- TTL, invalidación explícita y expulsión por capacidad son cosas distintas. Lo habitual es invalidar al escribir y dejar el TTL como red de seguridad.
- Las claves incluyen todo lo que cambia la respuesta, con un sufijo de versión.
- Caché local frente a distribuida: la local diverge entre réplicas.
- Patrones: cache-aside, read-through, write-through, refresh-ahead, write-behind.
- Fallos a diseñar: cache stampede, cold start y caída de la caché.
- Métricas mínimas: hits y misses, latencia con y sin caché, errores, evicciones, y carga de la fuente antes y después.

### Clase 4 — Comunicación asíncrona

- Lo asíncrono sirve para efectos que pueden terminar después; lo que decide la respuesta actual va síncrono.
- Evento (un hecho que ya ocurrió), comando (se pide una acción) y tarea son cosas distintas.
- Cola de trabajo con consumidores que compiten; publicación y suscripción con una cola por suscriptor.
- El mensaje es un contrato: tipo, versión, identificador único, fecha, identificador de correlación y datos.
- Entrega al menos una vez; confirmar (ACK) después del efecto durable.
- Idempotencia por restricción única, por transición de máquina de estados o por clave idempotente.
- Reintentos con límite, espera creciente y jitter; mensajes venenosos a una cola de mensajes fallidos (DLQ) con dueño, alerta y procedimiento.
- Métricas: profundidad y antigüedad de la cola, tiempo hasta el efecto, reintentos, mensajes en DLQ.

### Clase 5 — Motores de búsqueda

- Un motor de búsqueda no reemplaza a la base: es una vista optimizada para encontrar y ordenar.
- Texto, filtro, faceta y ranking responden preguntas distintas.
- El índice es una **copia de lectura derivada**, actualizada por eventos (Outbox → broker → indexador).
- El indexador es un consumidor idempotente: identificador estable, upsert y versión monotónica para descartar mensajes viejos.
- Hay que definir y medir el atraso aceptable entre la fuente y el índice.
- El índice tiene que poder reconstruirse desde la fuente (reindexación).
- Si la búsqueda se cae, no debe impedir las operaciones de negocio.

### Clase 6 — API gateway, balanceo y descubrimiento

- Balanceador: elige una réplica sana entre copias iguales. Gateway: decide qué API se expone y bajo qué política. No son lo mismo.
- Algoritmos de balanceo: round-robin, con pesos, menos conexiones, menor tiempo de respuesta.
- Health checks: distinguir proceso vivo, instancia lista y dependencia degradada. Exigir varias verificaciones seguidas para evitar que una réplica entre y salga.
- Servicios sin estado; la sesión, fuera de la réplica.
- El gateway autentica; cada servicio sigue autorizando sobre sus propios recursos. No debe contener reglas de negocio.
- Límite de tráfico con token bucket; ante el exceso, responder 429. Con varias réplicas del gateway, el límite hay que coordinarlo.
- Descubrimiento de servicios, del lado del cliente o del servidor.

### Clase 7 — Resiliencia ante fallos parciales

- La lentitud puede ser peor que un error: retiene recursos y provoca fallas en cascada.
- Timeouts y deadlines que se propagan entre capas. Un timeout no significa que la operación no ocurrió: el resultado puede ser desconocido.
- Reintentar solo fallos transitorios, con límite, espera creciente y jitter, y solo en una capa.
- Claves de idempotencia para repetir una mutación sin duplicar su efecto.
- Circuit breaker (cerrado, abierto, semiabierto), bulkhead y fallback. El fallback tiene que ser honesto: no se inventan datos.
- El frontend muestra estados de negocio, incluido "verificando", y no un error genérico.
- La resiliencia se demuestra provocando fallas acotadas y midiendo.

### Estilos de arquitectura de backend

- **Capas:** handler → servicio → repositorio. Válido cuando hay pocas reglas.
- **DDD:** lenguaje del negocio, contextos delimitados, entidades, objetos de valor, agregados e invariantes. Es una forma de modelar, no una estructura de carpetas.
- **Hexagonal:** un núcleo con las reglas, puertos de entrada y salida, y adaptadores. El núcleo no conoce el framework ni la base.
- **Clean Architecture:** la misma intención, en anillos con dependencias hacia adentro.
- **CQRS:** separar comandos y consultas; puede empezar con una sola base.
- **Event Sourcing:** guardar los hechos y reconstruir el estado; el más costoso.
- **Reactivo:** entrada y salida no bloqueante y control de demanda.
- No se adoptan en conjunto: cada uno se justifica por un problema concreto.
