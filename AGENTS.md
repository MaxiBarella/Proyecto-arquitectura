# Reglas para agentes

Estas reglas valen para cualquier agente de IA que trabaje en este repo, en tareas de análisis, diseño, documentación o código.

## Las decisiones las toma el equipo

Toda decisión de arquitectura o del proyecto la toman los integrantes del grupo. El agente no decide ni da nada por sentado.

Cuando el trabajo llega a un punto donde hay que elegir:

1. **Frenar y consultar** antes de avanzar sobre ese punto.
2. **Presentar las opciones posibles**, con qué implica cada una, sus ventajas y sus costos, y cómo se relaciona con el enunciado.
3. Se puede **dar una recomendación**, marcada como tal. La elección es del equipo.
4. **Esperar la respuesta.** No seguir como si la opción recomendada ya estuviera elegida.
5. Una vez decidido, **registrarlo** donde corresponda (ADR en `docs/adr/`, documentación de alcance o arquitectura).

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

## Contexto

- El enunciado analizado está en [docs/enunciado-tp-integrador.md](docs/enunciado-tp-integrador.md).
- Las notas de la clase del 2/10 están en [docs/requisitos-del-profe.md](docs/requisitos-del-profe.md).
- La defensa final es individual: cada integrante tiene que poder explicar y justificar cada decisión. Por eso las explicaciones tienen que alcanzar para que el equipo entienda lo que elige.
