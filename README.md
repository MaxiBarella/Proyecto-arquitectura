# Gestión de arribos y despegues de un aeropuerto

Práctico Integrador 2026 de Arquitectura de Software, Facultad de Ingeniería, Universidad Católica de Córdoba.

**Integrantes:** Maximo Barella, Salvador Solana, Malena Griffi y Carola Jalil.

> **Estado: presentación grupal (11/11/2026).** El sistema funciona de punta a punta: cuatro servicios detrás de un gateway, con su interfaz web, eventos, búsqueda, caché, balanceo, observabilidad y pruebas de carga. Clima, la capacidad que publicamos, corre en la nube.

## El dominio

Un aeropuerto tiene pocas pistas y muchos vuelos que quieren usarlas. Cada vuelo necesita un turno de pista, un **slot**, para aterrizar o para despegar, y en un mismo momento una pista admite un solo avión.

El sistema administra las pistas de **un aeropuerto**, el de Córdoba. Participan tres tipos de usuario:

- La **aerolínea** publica sus vuelos, que pueden ser arribos o despegues, y pide un slot para cada uno.
- El **operador del aeropuerto** administra las pistas: las da de alta, las cierra y las reabre.
- El **visitante**, por ejemplo un pasajero, consulta el tablero de arribos y partidas sin iniciar sesión.

## El objetivo

Asignar las pistas sin que dos vuelos queden nunca con el mismo slot, aun cuando varios lo pidan al mismo tiempo, y resolver de forma ordenada qué pasa con los vuelos cuando el clima obliga a cerrar una pista.

## El flujo principal

1. La aerolínea **publica un vuelo** e indica si es un arribo o un despegue. El vuelo queda pendiente de slot.
2. La aerolínea **pide un slot**: una pista y un bloque horario.
3. Si el slot está libre, el vuelo queda **programado**. Si está ocupado, el pedido se rechaza y se informan los slots libres más cercanos.
4. **Los arribos tienen prioridad.** Si un arribo pide un slot que tiene un despegue, se lo queda, y el despegue pasa a demorado.
5. El vuelo aparece en el **tablero público**, que se actualiza con cada cambio de estado.

Cuando el clima deja de permitir la operación, o cuando el operador lo decide, **la pista se cierra**:

- los despegues afectados quedan **demorados**, y la aerolínea les pide un slot nuevo;
- los arribos afectados pasan a **desviados** al aeropuerto alternativo más cercano que esté disponible.

Las reglas completas, los estados de un vuelo y los criterios de aceptación están en [SPEC.md](SPEC.md).

## Cómo está armado

El sistema son cuatro microservicios en Go detrás de un API gateway, con una interfaz web en React.

| Servicio | Qué hace | Guarda en | Organización interna |
|---|---|---|---|
| **Vuelos** | Publica los arribos y despegues, sigue su estado y resuelve el tablero. Corre en dos instancias. | MySQL, y el tablero en Solr con caché en Memcached | CQRS |
| **Pistas** | Asigna los slots sin que se pisen, y cierra y reabre las pistas. | MySQL | Hexagonal |
| **Clima** | Consulta Open-Meteo, informa las condiciones y si se puede operar. Es la capacidad que se publica para otro grupo. | MongoDB | Capas |
| **Usuarios** | Guarda los usuarios y entrega los tokens de sesión. | MySQL | Capas |

Alrededor de los servicios:

- **NGINX** es el punto de entrada: sirve la interfaz, verifica el token, manda cada pedido a su servicio, limita el tráfico y reparte entre las dos instancias de Vuelos ([ADR-012](docs/adr/ADR-012.md), [nginx/](nginx/README.md)).
- **RabbitMQ** lleva los eventos entre servicios, con outbox, consumidores idempotentes y cola de mensajes fallidos ([ADR-005](docs/adr/ADR-005.md)).
- **Grafana** (otel-lgtm) junta los logs, las métricas y las trazas, con el tablero y las alertas guardados en el repo ([ADR-011](docs/adr/ADR-011.md)).
- **Recién hecho**, el proveedor de otro grupo, manda el mail a la aerolínea cuando el clima le demora o le desvía un vuelo ([ADR-009](docs/adr/ADR-009.md)). En local se usa su mock.

El detalle, con los diagramas de contexto y de contenedores, las limitaciones conocidas y la deuda técnica, está en [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Cómo ejecutarlo localmente

Hace falta Docker instalado y en marcha.

1. **Crear el `.env`** a partir del ejemplo y completarlo. Las contraseñas no están en el repo, que es público:

   ```bash
   cp .env.example .env
   ```

   - `JWT_SECRET` tiene que tener al menos 32 bytes; si no, NGINX no arranca. Se genera con `openssl rand -base64 32`.
   - `USUARIOS_OPERADOR_PASSWORD`, `USUARIOS_AEROLINEAS_PASSWORD` y `USUARIOS_LATAM_PASSWORD` son las contraseñas de las cuentas de prueba.
   - `AEROLINEAS_EMAILS` dice a qué mail se avisa a cada aerolínea.

2. **Levantar todo:**

   ```bash
   docker compose up --build
   ```

3. **Entrar.** Todo pasa por el gateway:

   | Qué | Dirección |
   |---|---|
   | La interfaz, con el tablero público | <http://localhost:8080> |
   | La API, por el gateway | `http://localhost:8080/api/vuelos/…`, `/api/pistas/…`, `/api/clima/…`, `/api/usuarios/…` |
   | Grafana: tablero "Aeropuerto: estado general", logs, métricas y trazas | <http://localhost:3001> |
   | RabbitMQ, la consola de las colas | <http://localhost:15672> |
   | Solr, el índice del tablero | <http://localhost:8983> |

   Las cuentas de prueba son `aerolineas` (aerolínea AR), `latam` (aerolínea LA) y `operador`, con las contraseñas del `.env`. Los servicios también exponen su puerto directo (Vuelos 8081, Pistas 8082, Clima 8083 y Usuarios 8084), solo para depurar: ahí no hay token y la identidad va en los encabezados `X-User-Id`, `X-User-Role` y `X-Airline`.

### Recorrido del flujo principal

1. **Aerolínea.** Entrar como `aerolineas`, publicar un vuelo y pedirle un slot. Si el slot está ocupado, la respuesta trae los 3 libres más cercanos.
2. **Tablero.** En la portada aparece el vuelo. Un cambio tarda hasta 2 minutos en verse: hasta 60 segundos del índice y hasta 60 de la caché.
3. **Operador.** Entrar como `operador` y forzar una tormenta. La pista se cierra, los despegues afectados quedan demorados y los arribos, desviados. La aerolínea recibe el mail, que en local se ve en los logs de `notificaciones-mock`. Al quitar la condición forzada, Clima vuelve a consultar Open-Meteo.
4. **Observar.** En Grafana se ve el pedido de slot con su traza de punta a punta, el reparto entre `vuelos-1` y `vuelos-2`, y los aciertos de la caché.

## Clima en la nube

La capacidad que publicamos corre en Render con MongoDB Atlas ([ADR-015](docs/adr/ADR-015.md)):

```bash
curl -H "X-API-Key: <clave del grupo>" https://clima-aeropuerto.onrender.com/airports/SACO/conditions
```

Cada grupo consumidor tiene su clave, con un límite de 60 pedidos por minuto. El primer pedido después de un rato sin uso puede tardar, porque el plan gratuito de Render apaga el servicio. El contrato, la guía de uso, los errores y cómo levantar el mock están en [docs/contracts/](docs/contracts/README.md).

## Tests

- **De cada servicio:** `go test ./...` dentro de su carpeta en [services/](services/). Los de integración levantan las dependencias reales con testcontainers (MySQL, MongoDB, Memcached y el mock del proveedor) y necesitan Docker abierto.
- **De carga:** los escenarios con k6 y cómo correrlos están en [tests/carga/](tests/carga/README.md). Los resultados están en el [ADR-013](docs/adr/ADR-013.md) y, los de la caché, en el [ADR-007](docs/adr/ADR-007.md).

## Documentación

| Documento | Qué contiene |
|---|---|
| [SPEC.md](SPEC.md) | Alcance: funcionalidades, reglas de negocio, estados y criterios de aceptación. Es el documento que manda. |
| [docs/decisiones.md](docs/decisiones.md) | Todo lo que decidió el equipo, con quién y por qué. |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Arquitectura general, diagramas, limitaciones conocidas y deuda técnica aceptada. |
| [docs/adr/](docs/adr/README.md) | Las decisiones de arquitectura, una por archivo, con su estado. |
| [docs/POSTMORTEM.md](docs/POSTMORTEM.md) | La caída provocada de Pistas y de RabbitMQ, con su evidencia. |
| [docs/resiliencia.md](docs/resiliencia.md) | Qué pasa cuando se cae Pistas, el mecanismo que lo contiene y su evidencia. |
| [docs/contracts/](docs/contracts/README.md) | El contrato de Clima, la capacidad publicada, con su guía y su mock. |
| [services/](services/) | El código de cada servicio; cada carpeta tiene su `README.md` y Vuelos y Pistas, su `openapi.yaml`. |
| [frontend/](frontend/README.md) | La interfaz web y sus pantallas. |
| [docs/enunciado-tp-integrador.md](docs/enunciado-tp-integrador.md) | El enunciado del práctico, analizado, con lo que pide cada entrega. |
| [docs/entrega-1.md](docs/entrega-1.md) y [docs/entrega-2.md](docs/entrega-2.md) | Qué produjo cada integrante en cada entrega. |
| [docs/contenido-de-la-materia.md](docs/contenido-de-la-materia.md) | Herramientas y temas vistos en la materia. |

## Forma de trabajo

`main` es la rama común y la que se evalúa. Cada cambio se trabaja en una rama propia y entra a `main` por un pull request que revisa otro integrante. `main` está protegida: no admite cambios directos.

El detalle está en [docs/reglas-de-versionado.md](docs/reglas-de-versionado.md).

## Uso de inteligencia artificial

El proyecto se trabaja con asistentes de IA, con una regla central: **toda decisión de arquitectura o de proyecto la toma el equipo**. El asistente presenta las opciones con sus costos y espera la respuesta; no decide.

- Las reglas que siguen los asistentes están en [AGENTS.md](AGENTS.md).
- Cada decisión queda registrada en [docs/decisiones.md](docs/decisiones.md) antes de escribirse la documentación o el código que depende de ella.
- Los cambios hechos con un asistente llevan la marca `Co-Authored-By` en el commit.

El repositorio tiene instalado [BMAD Method](https://github.com/bmad-code-org/BMAD-METHOD) (v6.12.0), en `_bmad/` y `.claude/skills/`. Sus artefactos se generan a partir de la documentación del repo, nunca al revés; hasta ahora no se generó ninguno en `_bmad-output/`.
