# Decisiones de arquitectura (ADR)

Un ADR es una ficha corta que deja por escrito una decisión de arquitectura: qué se decidió, por qué, y qué otras opciones se descartaron. Sirve para que, más adelante, cualquiera pueda entender el motivo de una decisión sin tener que preguntarle a quien la tomó.

El enunciado pide trece obligatorios, D1 a D13, cada uno en un archivo aparte.

## Cómo se escribe uno

1. Buscá en la tabla de abajo el número que le corresponde.
2. Copiá [PLANTILLA.md](PLANTILLA.md) con el nombre `ADR-XXX.md`, en esta carpeta.
3. Completalo con lo que el equipo decidió en [../decisiones.md](../decisiones.md). Las opciones que ahí se descartaron son las "alternativas evaluadas".
4. Actualizá la columna "Estado" de la tabla de abajo, en el mismo pull request.

No se inventan decisiones al escribir un ADR. Si falta algo por decidir, se lleva al grupo y, mientras tanto, se anota en la sección "Pendiente".

## Reglas

- **Numeración.** El número del ADR es el de la decisión del enunciado: D1 es `ADR-001`, D8 es `ADR-008`. Los ADR adicionales, que no están en el enunciado, empiezan en `ADR-014`.
- **Nombre del archivo.** `ADR-XXX.md`, con tres dígitos, como lo escribe el enunciado.
- **Mismo formato.** Todos usan la plantilla, con las mismas secciones y en el mismo orden.
- **No se borran.** Si una decisión cambia, el ADR anterior queda con el estado "Reemplazado por ADR-XXX" y se escribe uno nuevo, con el siguiente número libre, que indica a cuál reemplaza y por qué.
- **Sí se completan.** Un ADR en "versión inicial" se puede ampliar y validar sin crear otro, siempre que la decisión de fondo no cambie.

## Índice

| ADR | Decisión | Qué tiene que quedar justificado | Sale de | Responsable | Vence | Estado |
|---|---|---|---|---|---|---|
| [ADR-001](ADR-001.md) | D1. Límites de los servicios | Criterios para separar responsabilidades, relaciones entre servicios y propiedad de los datos. | 3.1 a 3.3 | Carola | Entrega 1 | Aceptado |
| ADR-002 | D2. Arquitectura interna de los servicios | Estilos o patrones elegidos, dependencias internas y por qué cada uno se adecua a su servicio. | 7.3, 7.5, 7.6 | A asignar | Entrega 2 | Sin escribir |
| [ADR-003](ADR-003.md) | D3. Persistencia | Almacenamiento de cada servicio, patrones de acceso que soporta y limitaciones aceptadas. | 5.1 a 5.5, 7.2 | Salvador | Entrega 1, versión inicial | Versión inicial |
| ADR-004 | D4. Consistencia y concurrencia | Operaciones con garantías especiales, y tratamiento de concurrencia, duplicación, pérdida y fallas parciales. | 2.3, 2.7, 4.4 | A asignar | Sin entrega asignada | Sin escribir |
| [ADR-005](ADR-005.md) | D5. Comunicación entre servicios | Interacciones síncronas y asíncronas, timeouts, reintentos, eventos e idempotencia. | 4.1 a 4.9 | Salvador | Entrega 1, versión inicial | Versión inicial |
| ADR-006 | D6. Búsqueda | Qué se indexa, criterios de consulta, actualización o reconstrucción del índice y retraso aceptable. | 3.2 | A asignar | Entrega 2 | Sin escribir |
| ADR-007 | D7. Caché | Qué se guarda, invalidación, vigencia, comportamiento ante fallas y criterios de medición. | Sin decidir | A asignar | Entrega 2 | Sin escribir |
| [ADR-008](ADR-008.md) | D8. Contrato propio | Diseño, publicación, compatibilidad y estrategia de versionado de la capacidad que se ofrece. | 6.1 a 6.19 | Male | Entrega 1 | Aceptado |
| ADR-009 | D9. Consumo del proveedor | Cómo se incorpora la capacidad externa, adaptación al contrato y comportamiento ante errores, cambios o indisponibilidad. | Sin decidir | A asignar | Entrega 2 | Sin escribir |
| ADR-010 | D10. Resiliencia | Comportamiento ante fallas y mecanismos para limitar su propagación o degradar de forma controlada. | Sin decidir | A asignar | Entrega 2 | Sin escribir |
| ADR-011 | D11. Observabilidad | Estrategia de logs, métricas, trazas y tableros, objetivo de servicio y criterios de alerta. | Sin decidir | A asignar | Entrega 2, primera versión | Sin escribir |
| ADR-012 | D12. Balanceo de carga | Servicio elegido, estrategia de distribución, verificación de disponibilidad y caída de una instancia. | Sin decidir | A asignar | Entrega 2 | Sin escribir |
| ADR-013 | D13. Capacidad y costos | Resultados de las pruebas de carga, principal límite, alternativa de escalado y estimación de costos. | Sin decidir | A asignar | Entrega 2 | Sin escribir |

La columna "Sale de" indica los números de [../decisiones.md](../decisiones.md) que alimentan cada ADR.

Estados posibles de la última columna: Sin escribir, Versión inicial, Aceptado, Reemplazado.
