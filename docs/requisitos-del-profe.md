# Requisitos del proyecto según la explicación del profe

Fuentes:

1. Grabación de la clase del 2/10/2026 (26 min), transcripta automáticamente. Arranca con la explicación ya empezada y el audio es malo en varios tramos.
2. Notas tomadas en clase por una compañera, copiadas tal cual al final de este documento.

Lo marcado con **(a confirmar)** hay que chequearlo contra la consigna escrita.

## Requisitos técnicos

| Tema | Qué pide |
|---|---|
| Microservicios | Mínimo 3. El máximo es libre (4, 5, 6). |
| Bases de datos | Dos: una relacional y una no relacional. Nosotros decidimos dónde va cada una. Además, una caché. |
| Balanceador de carga | Obligatorio. Hay que justificar cuál se usa, explicar cómo funciona y cómo maneja los errores. |
| Manejo de errores | Es criterio para todo: cada componente tiene que decir qué pasa cuando falla. |
| Motor de búsqueda | Hay que tener uno. Se indexan solo los datos importantes para buscar, no todo. |
| Comunicación | Sincrónica y asincrónica (mensajería entre servicios), las dos con buen manejo de errores. |
| No reinventar la rueda | Caché, motor de búsqueda y mensajería se resuelven con herramientas existentes. Hacer una caché propia desde cero desaprueba. |
| Consistencia | Al menos en la operación más importante: cuando cambia un dato en la base, la caché y el motor de búsqueda se actualizan después (por ejemplo con un evento). Hay que definir el tiempo de actualización y justificarlo, y explicar cómo se evita que la caché devuelva datos incorrectos. |
| Resiliencia | Si un servicio se cae, el resto sigue funcionando. Elegir una función importante, manejar su caída y tener un archivo con los logs y la explicación de cómo se gestiona. |
| Observabilidad | Logs con estructura fácil de leer, métricas y un tablero donde verlas (ejemplo: cuántos 404 hubo la semana pasada). El tema se da en la próxima clase teórica. |
| Tests | Tests de integración y tests de carga con picos (de 10 a 1000 pedidos por segundo) para ver si el balanceador balancea y cómo se la banca la caché. |
| Git | Historial en Git con un método de trabajo explicado. No controlan cuál, pero no puede ser cualquier cosa. |
| Puesta en marcha | README con pasos claros. Tiene que correr fácil en cualquier computadora; si al profe no le corre, es un problema. |

## Integración entre grupos

- Publicamos una funcionalidad (un endpoint) documentada para que otro grupo la consuma.
- Consumimos la funcionalidad de un tercer grupo, guiándonos solo por su documento, sin hablar con sus desarrolladores.
- Solo esa funcionalidad compartida tiene que estar desplegada en internet, no corriendo local.
- Hosting gratuito y simple: Vercel, Netlify, Render, Railway, Cloudflare. Nada de AWS ni Azure.
- La idea es experimentar lo que pasa en el trabajo real, donde se integran servicios de terceros (pagos con Mercado Pago, mapas con Google Maps).
- Consejo del profe: cuanto más simple sea lo que se publica, mejor. Conviene separarlo en un microservicio chico en vez de dejarlo dentro de uno complejo.
- Tiene que ser lo primero que terminemos, para no trabar al grupo que depende de nosotros.

## Documentación y uso de IA

- Se puede y se recomienda usar IA, pero con una metodología basada en especificaciones, no pidiéndole "haceme el proyecto".
- Orden esperado: reglas de negocio, decisiones de arquitectura, contexto, arquitectura de alto y bajo nivel, estructura de los microservicios y su organización interna.
- Decisiones de arquitectura registradas con historial y fecha. Si una decisión cambia, no se borra la anterior: se agrega la nueva. Lo mismo para cambios en reglas de negocio.
- Las decisiones significativas se registran antes de programar, antes de que la IA las tome por nosotros.
- También hay que documentar cómo se usó la IA.
- Documentos obligatorios: cómo correr el proyecto, reglas de negocio, decisiones de arquitectura, funcionalidad compartida, gestión de la caída.
- Los nombres y la estructura de carpetas son libres, pero todo tiene que estar en la documentación del repo.

## Dominio

- No se puede repetir entre los grupos de las tres comisiones de práctico.
- Ejemplos que usó el profe (conviene evitarlos): venta de entradas para eventos, sistema médico.

## Entregas

| Fecha | Qué hay que llevar |
|---|---|
| 2/10 | Definir el dominio y crear el repo. Agregar a los cinco profes como colaboradores. |
| 9/10 **(a confirmar)** | Diagramas de contexto y de contenedores, cantidad de microservicios y bases de datos, la funcionalidad que vamos a publicar, tres microservicios levantados y comunicándose aunque no hagan nada, y una primera versión de las decisiones de la tabla de la consigna. |
| 16/10 **(a confirmar)** | No se entrega nada. |
| 23/10 **(a confirmar)** | Entrega con presentación grupal. Algunas partes quedan para el final. |

Hay funcionalidades extra en la consigna, pero no salvan un proyecto flojo.

## Anexo: notas de clase de una compañera

> **Consejos o parte de la consigna:**
>
> - Balanceador de carga: debe tener justificación y explicación de ese balanceador usado. Cómo va a manejar el error (criterio para todo).
> - No indexar un dato en un motor de búsqueda que no tenga que ver, el motor guarda los datos más importantes.
> - Para la búsqueda, comunicación sincrónica y asincrónica: hay que tener buen manejo de errores.
> - Dos BD (nosotros vamos a decidir dónde usar cada una):
>   - relacional
>   - no relacional
> - NO inventar una caché.
> - Tiempo de actualización de los eventos (tal y por qué).
> - Resiliencia: si algo se cae el resto tiene que seguir funcionando. Que los errores se manejen. Elegir una funcionalidad importante, que tenga un archivo donde están los logs y la explicación de cómo se está gestionando la caída de ese error.
> - Mantenimiento: métricas y un lugar donde ver las métricas. Ver cuántos errores 404 ocurrieron la semana pasada.
> - Testing: gestionar test cuando ocurre un pico. 10 por segundo, 1000 por segundo para ver, por ejemplo, cómo se la banca la caché.
> - Tiene que haber un historial en GitHub.
> - Readme con los pasos del proyecto.
> - Debe ser fácil de correr en cualquier computadora.
> - Integración entre grupos: inventar una funcionalidad. Ejemplo: un endpoint para que alguien registre un evento, y un sistema médico quiere publicar un evento de un congreso. No tiene que correr solo en nuestra máquina. (Pago: Mercado Pago. Mapa: Google Maps.)
> - Documentación: TODO. El uso de la IA también.
>   - Buscar metodología para trabajar con IA.
>   - Reglas de negocio.
>   - Arquitectura (de alto y bajo nivel).
>   - Cómo están organizados los microservicios.
>   - Decisiones (ANTES DE QUE CLAUDE LAS HAGA).
>   - Si quería hacer tres microservicios y después cambié a 4, dejo constancia: tal fecha se decidió esto.
