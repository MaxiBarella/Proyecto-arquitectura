# Gestión de arribos y despegues de un aeropuerto

Práctico Integrador 2026 de Arquitectura de Software, Facultad de Ingeniería, Universidad Católica de Córdoba.

**Integrantes:** Maximo Barella, Salvador Solana, Malena Griffi y Carola Jalil.

> **Estado: Entrega 1, diseño y contrato con mock (9/10/2026).** Todavía no hay sistema ejecutable. Este repositorio contiene el alcance, las decisiones y el diseño.

## El dominio

Un aeropuerto tiene pocas pistas y muchos vuelos que quieren usarlas. Cada vuelo necesita un turno de pista, un **slot**, para aterrizar o para despegar, y en un mismo momento una pista admite un solo avión.

El sistema administra las pistas de **un aeropuerto**. Participan tres tipos de usuario:

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
- los arribos afectados pasan a **desviados** a otro aeropuerto.

Las reglas completas, los estados de un vuelo y los criterios de aceptación están en [SPEC.md](SPEC.md).

## Cómo está armado

El sistema son cuatro microservicios detrás de un API gateway, con una interfaz web.

| Servicio | Qué hace |
|---|---|
| **Vuelos** | Publica los arribos y despegues, sigue su estado y resuelve la búsqueda del tablero. |
| **Pistas** | Asigna los slots sin que se pisen, y cierra y reabre las pistas. |
| **Clima** | Informa las condiciones y si se puede operar. Es la capacidad que se publica para otro grupo. |
| **Usuarios** | Guarda los usuarios y entrega las credenciales de sesión. |

El detalle, con los diagramas, va en `docs/ARCHITECTURE.md` (en preparación).

## Cómo ejecutarlo localmente

Hace falta tener Docker instalado y en marcha.

```bash
cp .env.example .env
docker compose up --build
```

El primer comando crea el archivo de configuración local a partir del ejemplo; conviene cambiar las contraseñas. El segundo levanta los cuatro servicios y sus dependencias: tres MySQL, MongoDB, RabbitMQ y Solr.

Por ahora los servicios son la estructura inicial: arrancan y responden `GET /health`, sin lógica.

| Servicio | Dirección local |
|---|---|
| Vuelos | <http://localhost:8081/health> |
| Pistas | <http://localhost:8082/health> |
| Clima | <http://localhost:8083/health> |
| Usuarios | <http://localhost:8084/health> |

El código de cada servicio está en [services/](services/).

## Cómo acceder a lo desplegado

Todavía no hay nada desplegado. Acá va a figurar la dirección pública del servicio de Clima, que es la capacidad que consume otro grupo.

## Documentación

| Documento | Qué contiene | Estado |
|---|---|---|
| [SPEC.md](SPEC.md) | Alcance: funcionalidades, reglas de negocio, estados y criterios de aceptación. | Primera versión |
| [docs/decisiones.md](docs/decisiones.md) | Lo que el equipo decidió y lo que quedó sin cerrar. | Al día |
| `docs/ARCHITECTURE.md` | Arquitectura general, con diagramas de contexto y de contenedores. | En preparación |
| `docs/adr/` | Decisiones de arquitectura, una por archivo. | En preparación |
| `docs/contracts/` | Contrato y documentación de la capacidad publicada, el servicio de Clima. | En preparación |
| `docs/POSTMORTEM.md` | Informe de la caída provocada. | Entregas posteriores |
| [docs/entrega-1.md](docs/entrega-1.md) | Qué produce cada integrante para la Entrega 1. | Al día |
| [docs/enunciado-tp-integrador.md](docs/enunciado-tp-integrador.md) | El enunciado del práctico, analizado. | Al día |
| [docs/contenido-de-la-materia.md](docs/contenido-de-la-materia.md) | Herramientas y temas vistos en la materia. | Al día |

## Forma de trabajo

`main` es la rama común y la que se evalúa. Cada cambio se trabaja en una rama propia y entra a `main` por un pull request que revisa otro integrante. `main` está protegida: no admite cambios directos.

El detalle está en [docs/reglas-de-versionado.md](docs/reglas-de-versionado.md).

## Uso de inteligencia artificial

El proyecto se trabaja con asistentes de IA, con una regla central: **toda decisión de arquitectura o de proyecto la toma el equipo**. El asistente presenta las opciones con sus costos y espera la respuesta; no decide.

- Las reglas que siguen los asistentes están en [AGENTS.md](AGENTS.md).
- Cada decisión queda registrada en [docs/decisiones.md](docs/decisiones.md) antes de escribirse la documentación o el código que depende de ella.
- Los cambios hechos con un asistente llevan la marca `Co-Authored-By` en el commit.

El repositorio tiene instalado [BMAD Method](https://github.com/bmad-code-org/BMAD-METHOD) (v6.12.0), en `_bmad/` y `.claude/skills/`. Se va a usar cuando arranque el desarrollo, alimentado con la documentación de este repositorio.
