# Reglas para agentes

Estas reglas valen para cualquier agente de IA que trabaje en este repo, en tareas de análisis, diseño, documentación o código.

## Las decisiones las toma el equipo

Toda decisión de arquitectura o del proyecto la toman los integrantes del grupo. El agente no decide ni da nada por sentado.

Cuando el trabajo llega a un punto donde hay que elegir:

1. **Frenar y consultar** antes de avanzar sobre ese punto.
2. **Presentar las opciones posibles**, con qué implica cada una, sus ventajas y sus costos, y cómo se relaciona con el enunciado.
3. Se puede **dar una recomendación**, marcada como tal. La elección es del equipo.
4. **Esperar la respuesta.** No seguir como si la opción recomendada ya estuviera elegida.
5. Una vez decidido, **registrarlo** donde corresponda (ADR en `docs/adr/`, documentación de alcance o arquitectura). Los ADR se escriben copiando [docs/adr/PLANTILLA.md](docs/adr/PLANTILLA.md), sin cambiar sus secciones, y con el número que indica [docs/adr/README.md](docs/adr/README.md).

Cuenta como decisión, entre otras cosas:

- límites, cantidad y responsabilidades de los servicios, y quién es dueño de cada dato;
- lenguajes, frameworks, librerías, bases de datos, motor de búsqueda, caché, mensajería, gateway, balanceador y herramientas de observabilidad;
- patrones de arquitectura interna y estructura de carpetas;
- reglas de negocio, estados, validaciones y alcance funcional;
- contratos: endpoints, eventos, formatos, errores y versionado;
- timeouts, reintentos, vigencia de caché, retraso tolerable del índice, umbrales y cualquier otro valor con consecuencias;
- estrategia de ramas, de tests y de despliegue.

Si una tarea ya decidida abre una elección nueva que no estaba contemplada, también se consulta. Ante la duda de si algo es una decisión, se pregunta.

Lo que el agente dijo o propuso en una conversación no es una decisión hasta que el equipo la confirma. Solo vale como decidido lo que está registrado en el repo.

## Si te preguntan "¿qué tengo que hacer?"

1. Identificá quién es la persona: mirá `git config user.name` o su usuario de GitHub. Si no queda claro, preguntale el nombre.
2. Abrí [docs/entrega-1.md](docs/entrega-1.md), buscá su paquete en la tabla y contale lo que dice su sección: qué produce, de qué decisiones sale, qué falta decidir y quién lo revisa.
3. Decile también lo que vale para todos: rama propia, pull request y no tocar los archivos de otro paquete.
4. No arranques a escribir nada hasta que te lo pida. Si su paquete tiene cosas sin decidir, avisale que hay que llevarlas al grupo.

## Documentación y BMAD

- El alcance del sistema está en [SPEC.md](SPEC.md), en la raíz. Es el documento que manda sobre funcionalidades, reglas de negocio y criterios de aceptación.
- Los artefactos de BMAD en `_bmad-output/` se **generan a partir** de la documentación del repo (`SPEC.md`, `docs/decisiones.md`, `docs/ARCHITECTURE.md` y los ADR). No se escriben ni se corrigen a mano.
- Si una regla cambia, se cambia primero en el documento original y después se vuelve a generar el artefacto de BMAD. Nunca al revés.
- Hasta la Entrega 1 no se generan artefactos de BMAD: se usan cuando arranque el desarrollo.

## Versionado

Las reglas de ramas, commits y pull requests están en [docs/reglas-de-versionado.md](docs/reglas-de-versionado.md). El agente trabaja en una rama propia, no sube a `main` y no mergea por su cuenta.

## Contexto

- El alcance, las reglas de negocio y los criterios de aceptación están en [SPEC.md](SPEC.md).
- Lo que el equipo ya decidió está en [docs/decisiones.md](docs/decisiones.md). Antes de proponer algo, verificar que no esté decidido ahí.
- El trabajo de cada integrante para la entrega en curso está en [docs/entrega-1.md](docs/entrega-1.md).
- El enunciado analizado está en [docs/enunciado-tp-integrador.md](docs/enunciado-tp-integrador.md).
- Lo que se vio en la materia está en [docs/contenido-de-la-materia.md](docs/contenido-de-la-materia.md).
- Las notas de la clase del 2/10 están en [docs/requisitos-del-profe.md](docs/requisitos-del-profe.md).
- La defensa final es individual: cada integrante tiene que poder explicar y justificar cada decisión. Por eso las explicaciones tienen que alcanzar para que el equipo entienda lo que elige.
