# Reglas de versionado

Cómo trabajamos con Git y GitHub en este repo. Valen para todos los integrantes y para los agentes de IA que trabajen por ellos.

Cubre lo que pide el enunciado en 2.12: una estrategia de ramas, revisiones e integración definida y usada.

## En corto

1. Cada funcionalidad se trabaja en **una rama propia**.
2. Se integra a `main` con un **pull request**.
3. **Otro integrante** lo revisa, lo aprueba y lo mergea. Durante el armado inicial, el autor puede mergear lo suyo.
4. Nadie sube cambios directo a `main`.

## Ramas

- `main` es la rama común y la que evalúa la cátedra. Tiene que estar siempre en un estado que se pueda levantar y mostrar.
- Cada funcionalidad, arreglo o documento va en una rama nueva, creada desde `main` actualizado.
- Una rama, un tema. Si aparece otra cosa para hacer, va en otra rama.
- Las ramas duran poco: conviene abrir el pull request en uno o dos días. Cuanto más vive una rama, más conflictos trae.
- Después del merge, la rama se borra.

`main` está protegida en GitHub desde el 7/10/2026: rechaza los push directos, exige que todo entre por pull request, y no admite force push ni que se borre. Vale también para el dueño del repo. GitHub no exige la aprobación de otro integrante: esa parte es un acuerdo del equipo.

### Nombres

`tipo/descripcion-corta`, en minúsculas y con guiones.

| Tipo | Para qué | Ejemplo |
|---|---|---|
| `feat` | Funcionalidad nueva | `feat/asignacion-de-slots` |
| `fix` | Corrección de un error | `fix/timeout-consulta-clima` |
| `docs` | Documentación y ADRs | `docs/adr-limites-de-servicios` |
| `test` | Tests sin cambio de comportamiento | `test/carga-tablero` |
| `refactor` | Reordenar código sin cambiar lo que hace | `refactor/capas-servicio-vuelos` |
| `chore` | Configuración, dependencias, infraestructura | `chore/docker-compose-inicial` |

## Commits

- En español, empezando con un verbo que diga qué hace el commit: "Agrega…", "Corrige…", "Actualiza…".
- Un commit, un cambio. Mejor varios commits chicos que uno enorme.
- La primera línea es corta (hasta unos 70 caracteres). Si hace falta explicar el porqué, va en el cuerpo, separado por una línea en blanco.
- No se commitea código que no levanta o con tests rotos, salvo en un pull request marcado como borrador.

## Pull requests

### Quien lo abre

- Lo apunta a `main`.
- Antes de abrirlo, actualiza su rama con `main` y resuelve los conflictos.
- Escribe en la descripción **qué cambia y por qué**, y cómo probarlo.
- Lo mantiene chico. Un pull request que se revisa en diez minutos se revisa bien; uno de mil líneas se aprueba sin leer.
- Si todavía no está listo pero quiere mostrarlo, lo abre como **borrador**.
- Responde los comentarios y hace los cambios en la misma rama.
- **No mergea su propio pull request de una funcionalidad.** La excepción es el armado inicial del proyecto (documentación base y configuración), donde el autor puede mergear lo suyo.

### Quien lo revisa

Tiene que ser un integrante distinto del autor. Revisa, y recién después aprueba y mergea.

Antes de aprobar verifica que:

- [ ] entiende qué cambia y podría explicarlo (la defensa es individual);
- [ ] el sistema levanta y los tests pasan;
- [ ] hay tests para las reglas de negocio que se agregan o cambian;
- [ ] la documentación afectada está actualizada en el mismo pull request;
- [ ] si hay una decisión de arquitectura, está su ADR;
- [ ] no hay secretos, contraseñas ni claves.

Si algo no cierra, pide cambios con un comentario concreto en vez de aprobar.

### Cambios que hace un agente de IA

- Siguen las mismas reglas: rama propia y pull request.
- El integrante que le pidió el trabajo es el autor y responde por el cambio. Lo lee antes de pedir revisión.
- El agente no mergea ni sube a `main` por su cuenta.
- Los commits hechos con un agente llevan la línea `Co-Authored-By` correspondiente, para que quede registro del uso de IA.

## Documentación y decisiones

- La documentación se actualiza **en el mismo pull request** que el cambio que la afecta, no después.
- Una decisión de arquitectura entra con su ADR en `docs/adr/`.
- Los ADR no se borran ni se reescriben. Si una decisión cambia, el ADR anterior se marca como reemplazado y se agrega uno nuevo que indica a cuál reemplaza y por qué.

## Secretos

- Nunca se commitean contraseñas, tokens, claves ni archivos `.env`. El `.gitignore` ya excluye `.env` y `.env.*`.
- Las variables que hacen falta se documentan en un `.env.example` con valores de ejemplo.
- Si un secreto se sube por error, no alcanza con borrarlo en otro commit: hay que cambiarlo y avisar al grupo.

## Entregas

Cada entrega se marca en `main` con un tag, para poder volver a ver qué se entregó:

| Tag | Fecha |
|---|---|
| `entrega-1` | 9/10/2026 |
| `entrega-2` | 23/10/2026 |
| `presentacion` | 11 o 13/11/2026 |

## Pendiente de definir

- **Cómo se mergea:** con commit de merge (conserva todos los commits de la rama y quién hizo cada uno) o con squash (deja un solo commit por pull request, historial más limpio pero se pierde el detalle).
- **Chequeos automáticos:** correr los tests en cada pull request.

El versionado del contrato de la capacidad publicada es otro tema: corresponde a la decisión D8 y va en `docs/contracts/`.
