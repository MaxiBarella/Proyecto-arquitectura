# SPEC — Gestión de arribos y despegues de un aeropuerto

Alcance, reglas de negocio y criterios de aceptación del sistema.

- **Versión:** 0.2, para la Entrega 1 (9/10/2026).
- **De dónde sale:** de las decisiones del equipo registradas en [docs/decisiones.md](docs/decisiones.md), puntos 1, 2.1 a 2.9, 6.7, 6.18 y 6.19. Este documento las desarrolla; no agrega decisiones nuevas.
- **Qué no cubre:** cómo se construye. Los servicios, las bases de datos y la comunicación están en `docs/ARCHITECTURE.md` y en los ADR de `docs/adr/`.

## 1. Propósito

El sistema administra el uso de las pistas de **un aeropuerto**, el de Córdoba. Las aerolíneas publican sus vuelos y piden, para cada uno, un turno de pista; el aeropuerto administra las pistas; cualquier persona puede consultar el tablero de arribos y partidas.

La pista es un recurso escaso: en un mismo momento, una pista admite un solo avión. El problema central del sistema es **asignar ese recurso sin que dos vuelos queden con el mismo turno**, incluso cuando varios lo piden a la vez, y resolver qué pasa cuando el clima obliga a cerrar una pista.

No es un sistema de altas, bajas y consultas: la acción principal, pedir un slot, tiene restricciones (un slot, un vuelo), prioridades (los arribos sobre los despegues), estados (el ciclo de vida del vuelo) y consecuencias (demoras y desvíos).

## 2. Glosario

| Término | Significado |
|---|---|
| **Vuelo** | Una operación de una aerolínea en nuestro aeropuerto. Es un arribo o un despegue. |
| **Despegue** | Vuelo que sale de nuestro aeropuerto. Nuestro aeropuerto es su origen. |
| **Arribo** | Vuelo que viene de otro aeropuerto y aterriza en el nuestro. Nuestro aeropuerto es su destino. |
| **Tipo de operación** | Si el vuelo es un arribo o un despegue. |
| **Pista** | Recurso físico donde se aterriza y se despega. Puede estar abierta o cerrada. |
| **Bloque** | Cada una de las partes iguales en que se divide el día de una pista. |
| **Slot** | El derecho de un vuelo a usar una pista durante un bloque. Sirve igual para aterrizar que para despegar. |
| **Aptitud operativa** | Lo que informa el servicio de clima: si las condiciones permiten operar o no. Es una sola para todo el aeropuerto, y vale igual para aterrizar que para despegar. |
| **Aeropuerto alternativo** | Aeropuerto cercano al que va un arribo desviado. |
| **Tablero** | La vista pública de arribos y partidas. |

## 3. Roles

| Rol | Inicia sesión | Qué hace |
|---|---|---|
| **Aerolínea** | Sí | Publica sus arribos y despegues, y pide un slot para cada uno. |
| **Operador del aeropuerto** | Sí | Administra las pistas: las da de alta, las cierra y las reabre. Indica qué aeropuertos alternativos están disponibles. |
| **Visitante** | No | Consulta el tablero de arribos y partidas. Es el caso del pasajero. |

No existe una cuenta de pasajero: quien quiere saber de un vuelo lo busca en el tablero.

## 4. Funcionalidades

### Aerolínea

| ID | Funcionalidad |
|---|---|
| F-01 | Iniciar sesión. |
| F-02 | Publicar un vuelo, indicando si es un arribo o un despegue, su origen, su destino y su horario. |
| F-03 | Pedir un slot para un vuelo, eligiendo pista y bloque. |
| F-04 | Ver los slots libres más cercanos cuando el que pidió no está disponible. |
| F-05 | Pedir un slot nuevo para un vuelo demorado. |
| F-06 | Cancelar un vuelo. |
| F-07 | Ver el estado de sus vuelos. |

### Operador del aeropuerto

| ID | Funcionalidad |
|---|---|
| F-08 | Iniciar sesión. |
| F-09 | Dar de alta una pista. |
| F-10 | Cerrar una pista a mano. |
| F-11 | Reabrir una pista. |
| F-12 | Forzar una condición climática, para probar y demostrar el cierre de pistas. Afecta solo a nuestro sistema: no cambia lo que se publica a otros grupos. |
| F-20 | Marcar un aeropuerto alternativo como no disponible, y volver a habilitarlo. |
| F-21 | Ver un aviso cuando el servicio de clima no puede obtener los datos. |

### Visitante

| ID | Funcionalidad |
|---|---|
| F-13 | Ver el tablero de arribos y partidas sin iniciar sesión. |
| F-14 | Buscar vuelos en el tablero con filtros, orden y paginación. |

### El sistema, sin intervención de nadie

| ID | Funcionalidad |
|---|---|
| F-15 | Cerrar una pista cuando el clima deja de permitir la operación, y reabrirla cuando vuelve a permitirla. |
| F-16 | Demorar los despegues y desviar los arribos afectados por el cierre de una pista, indicando a qué aeropuerto alternativo va cada arribo. |
| F-17 | Mantener el tablero al día con cada cambio de estado de un vuelo. |

### Hacia otros grupos

| ID | Funcionalidad |
|---|---|
| F-18 | Ofrecer a otro grupo las condiciones del clima y la aptitud operativa. El detalle está en `docs/contracts/`. |
| F-19 | Usar en un flujo importante la capacidad que publique el grupo proveedor. Todavía no fue asignada. |

## 5. Reglas de negocio

### Pistas y slots

| ID | Regla |
|---|---|
| RN-01 | El sistema administra las pistas de un solo aeropuerto: el de Córdoba, código ICAO `SACO`. |
| RN-02 | Las pistas se cargan como datos; el sistema arranca con dos. |
| RN-03 | El día de cada pista se divide en bloques fijos, todos de la misma duración. |
| RN-04 | Un slot es una pista más un bloque. **Un slot admite un solo vuelo.** |
| RN-05 | Un slot sirve igual para un arribo que para un despegue. |

### Asignación

| ID | Regla |
|---|---|
| RN-06 | Todo vuelo necesita un slot para quedar programado. Un vuelo recién publicado no tiene slot. |
| RN-07 | La aerolínea pide un slot exacto: una pista y un bloque. El sistema no elige por ella. |
| RN-08 | Si el slot está libre, el vuelo se lo queda. |
| RN-09 | Si el slot está ocupado y no corresponde aplicar la prioridad de RN-11, el pedido se rechaza y se informan los slots libres más cercanos. |
| RN-10 | Si dos pedidos por el mismo slot llegan al mismo tiempo, uno solo lo obtiene. El otro recibe la respuesta que le corresponda según RN-09 o RN-11. Nunca quedan dos vuelos con el mismo slot, y ninguna asignación se pierde. |

### Prioridad de los arribos

| ID | Regla |
|---|---|
| RN-11 | **Los arribos tienen prioridad sobre los despegues.** Si un arribo pide un slot ocupado por un despegue, el arribo se queda con el slot y el despegue lo pierde. |
| RN-12 | El despegue que pierde su slot por RN-11 pasa a demorado. |
| RN-13 | La prioridad vale solo de arribo sobre despegue. Entre dos arribos, o entre dos despegues, el slot es del que lo obtuvo primero. |
| RN-14 | Un despegue nunca le quita el slot a otro vuelo. |

### Cierre de pistas

| ID | Regla |
|---|---|
| RN-15 | Cuando el clima informa que no se puede operar, se cierran solas todas las pistas, para arribos y para despegues. La aptitud es una sola para todo el aeropuerto. |
| RN-16 | El operador también puede cerrar una pista a mano, por cualquier motivo. |
| RN-17 | Cuando se cierra una pista, los **despegues** afectados pierden su slot y pasan a demorado. |
| RN-18 | Cuando se cierra una pista, los **arribos** afectados pasan a desviado: aterrizan en un aeropuerto alternativo (RN-25) y dejan de ser una operación del nuestro. |
| RN-19 | Un vuelo demorado vuelve a quedar programado cuando la aerolínea le consigue un slot nuevo, con las mismas reglas de asignación. |
| RN-24 | Si el servicio de clima no puede obtener los datos, las pistas no se cierran ni se reabren solas: el operador ve un aviso y decide a mano. |

### Desvíos

| ID | Regla |
|---|---|
| RN-25 | Hay una lista de aeropuertos alternativos ordenada por distancia. Un arribo desviado va al primero de la lista que esté disponible. |
| RN-26 | El operador puede marcar un aeropuerto alternativo como no disponible, y entonces se saltea. El sistema no consulta el estado de los otros aeropuertos. |

### Vuelos

| ID | Regla |
|---|---|
| RN-20 | Todo vuelo es un arribo o un despegue. El tipo no cambia. |
| RN-21 | Un vuelo solo cambia de estado por las transiciones de la sección 6. Cualquier otra se rechaza. |
| RN-22 | Solo un arribo puede quedar desviado. |
| RN-23 | Cuando un vuelo se cancela, su slot queda libre. |

## 6. Estados de un vuelo

| Estado | Qué significa | ¿Es final? |
|---|---|---|
| **Pendiente de slot** | El vuelo fue publicado y todavía no tiene slot. | No |
| **Programado** | Tiene un slot asignado. | No |
| **Demorado** | Tenía slot y lo perdió. Espera uno nuevo. | No |
| **Finalizado** | La operación ocurrió. En el tablero se muestra como "Despegó" o "Aterrizó". | Sí |
| **Desviado** | El arribo no pudo aterrizar acá y fue a un aeropuerto alternativo. | Sí |
| **Cancelado** | El vuelo no se va a realizar. | Sí |

### Transiciones válidas

| De | A | Cuándo | Vale para |
|---|---|---|---|
| — | Pendiente de slot | La aerolínea publica el vuelo. | Arribos y despegues |
| Pendiente de slot | Programado | Se le asigna un slot. | Arribos y despegues |
| Programado | Demorado | Se cierra su pista (RN-17), o un arribo se queda con su slot (RN-12). | Solo despegues |
| Demorado | Programado | Se le asigna un slot nuevo. | Solo despegues |
| Programado | Desviado | Se cierra su pista (RN-18). | Solo arribos |
| Programado | Finalizado | La operación se realizó. | Arribos y despegues |
| Pendiente de slot, Programado o Demorado | Cancelado | Se cancela el vuelo. | Arribos y despegues |

Con las reglas actuales, un arribo nunca queda demorado: no pierde su slot ante otro vuelo (RN-13, RN-14) y, si se cierra su pista, se desvía (RN-18).

### Ejemplos de transiciones que se rechazan

- De Cancelado, Finalizado o Desviado a cualquier otro estado.
- De Pendiente de slot a Finalizado: no se puede operar sin slot.
- De Demorado a Finalizado: primero necesita un slot nuevo.
- Un despegue a Desviado.

## 7. Criterios de aceptación

Cada criterio indica la regla que comprueba.

### Asignación de slots

| ID | Regla | Criterio |
|---|---|---|
| CA-01 | RN-06 | Dado que una aerolínea publica un vuelo, entonces el vuelo queda en "Pendiente de slot" y aparece en el tablero. |
| CA-02 | RN-08 | Dado un vuelo pendiente y un slot libre, cuando la aerolínea pide ese slot, entonces el vuelo queda "Programado" y el slot figura ocupado por ese vuelo. |
| CA-03 | RN-09, RN-14 | Dado un slot ocupado por un vuelo cualquiera, cuando un despegue lo pide, entonces el pedido se rechaza, el slot sigue con su vuelo original y la respuesta incluye slots libres cercanos. |
| CA-04 | RN-09, RN-13 | Dado un slot ocupado por un arribo, cuando otro arribo lo pide, entonces el pedido se rechaza y el primer arribo conserva el slot. |
| CA-05 | RN-10 | Dados dos despegues que piden el mismo slot libre al mismo tiempo, entonces exactamente uno queda "Programado" con ese slot y el otro recibe un rechazo. |
| CA-06 | RN-10 | Dado un mismo pedido de slot enviado dos veces por un reintento, entonces el vuelo queda con un solo slot y no se ocupa ningún otro. |

### Prioridad de los arribos

| ID | Regla | Criterio |
|---|---|---|
| CA-07 | RN-11, RN-12 | Dado un slot ocupado por un despegue programado, cuando un arribo pide ese slot, entonces el arribo queda "Programado" con el slot y el despegue pasa a "Demorado" sin slot. |
| CA-08 | RN-10, RN-11 | Dados un arribo y un despegue que piden el mismo slot libre al mismo tiempo, entonces el slot termina asignado al arribo, y el despegue queda rechazado o demorado, pero nunca los dos con el slot. |
| CA-09 | RN-12 | Dado un despegue que perdió su slot ante un arribo, entonces el tablero lo muestra como "Demorado". |

### Cierre de pistas

| ID | Regla | Criterio |
|---|---|---|
| CA-10 | RN-15 | Dadas las pistas abiertas, cuando el clima pasa a no permitir la operación, entonces todas quedan cerradas sin que intervenga el operador. |
| CA-11 | RN-16 | Dada una pista abierta, cuando el operador la cierra a mano, entonces la pista queda cerrada aunque el clima permita operar. |
| CA-12 | RN-17 | Dada una pista con un despegue programado, cuando la pista se cierra, entonces el despegue pasa a "Demorado" y su slot queda libre. |
| CA-13 | RN-18 | Dada una pista con un arribo programado, cuando la pista se cierra, entonces el arribo pasa a "Desviado". |
| CA-14 | RN-19 | Dado un despegue demorado, cuando la aerolínea le pide un slot libre, entonces vuelve a "Programado". |
| CA-20 | RN-24 | Dado que el servicio de clima no puede obtener los datos, entonces ninguna pista cambia de estado sola, el operador ve un aviso y puede cerrar o reabrir pistas a mano. |
| CA-21 | RN-25 | Dado un arribo que pasa a "Desviado" con todos los alternativos disponibles, entonces se le asigna el más cercano de la lista. |
| CA-22 | RN-26 | Dado que el operador marcó como no disponible el alternativo más cercano, cuando un arribo pasa a "Desviado", entonces se le asigna el siguiente de la lista. |

### Estados y roles

| ID | Regla | Criterio |
|---|---|---|
| CA-15 | RN-21 | Dado un vuelo cancelado, finalizado o desviado, cuando se intenta cambiar su estado o asignarle un slot, entonces la operación se rechaza y el vuelo no cambia. |
| CA-16 | RN-22 | Dado un despegue, entonces ninguna operación lo deja en "Desviado". |
| CA-17 | RN-23 | Dado un vuelo programado, cuando se cancela, entonces su slot queda libre y otro vuelo puede pedirlo. |
| CA-18 | Sección 3 | Dado un visitante sin sesión, entonces puede ver y buscar en el tablero, y no puede publicar vuelos, pedir slots ni administrar pistas. |
| CA-19 | Sección 3 | Dada una aerolínea con sesión, entonces no puede dar de alta, cerrar ni reabrir pistas. |

## 8. Fuera de alcance

- **Ventas:** pasajes, reservas, pagos y cualquier cosa relacionada.
- **Pasajeros con cuenta:** no hay registro ni inicio de sesión de pasajeros, ni avisos personalizados.
- **Más de un aeropuerto:** no se coordinan slots con el aeropuerto del otro extremo del vuelo, ni se consulta el estado ni el clima de los aeropuertos alternativos.
- **Reasignación automática:** el sistema no le busca un slot nuevo a un vuelo demorado; lo pide la aerolínea.
- **Operación del día del vuelo:** embarque, puertas, rodaje, equipaje y control de tráfico aéreo.
- **Slots de duración variable** según el tipo de avión o de operación.

## 9. Pendiente de definir

Puntos que las decisiones tomadas no resuelven. Cada uno necesita una decisión del equipo antes de implementarse.

| # | Pendiente | Afecta a |
|---|---|---|
| 1 | Cuánto dura un bloque. | RN-03 |
| 2 | Qué se considera "slots libres más cercanos": cuántos se informan y hasta qué distancia. | RN-09, F-04 |
| 3 | Qué vuelos alcanza el cierre de una pista: todos los que tienen slot en ella de ahí en adelante, o solo los de un período. | RN-17, RN-18 |
| 4 | Si se puede pedir un slot en una pista cerrada. | RN-07 |
| 5 | Si una pista cerrada a mano por el operador se reabre sola cuando el clima lo permite, o solo la reabre el operador. | RN-15, RN-16, F-11 |
| 6 | Cómo pasa un vuelo a "Finalizado": lo marca alguien o es automático al pasar su bloque. | Sección 6 |
| 7 | Quién puede cancelar un vuelo, y si un arribo desviado puede además cancelarse. | F-06, RN-23 |
| 8 | Si una aerolínea puede cambiar el slot de un vuelo ya programado. | RN-19 |
| 9 | Si cada aerolínea ve y opera solo sus vuelos. | F-07 |
| 10 | Cómo se crean las cuentas de aerolínea y de operador. | F-01, F-08 |
| 11 | Qué filtros y qué orden ofrece la búsqueda del tablero. El enunciado exige paginación, filtros y al menos un orden. | F-14 |
| 12 | Qué datos exactos lleva un vuelo además de tipo, aerolínea, origen, destino, horario y estado. | F-02 |
| 13 | **Resuelto en la versión 0.2:** la aptitud operativa es una sola (decisión 6.7). | RN-15, F-18 |
| 14 | Qué capacidad del grupo proveedor se consume y en qué flujo entra. Depende de la asignación de la cátedra. | F-19 |
| 15 | Qué aeropuertos forman la lista de alternativos y qué servicio la guarda. | RN-25, F-20 |
| 16 | Qué pasa con un arribo desviado si ningún alternativo está disponible. | RN-25, RN-26 |
| 17 | Cómo se entera el sistema de que el servicio de clima no tiene datos, y qué pasa con una condición forzada mientras dura. | RN-24, F-12, F-21 |

## 10. Historial de cambios

Cuando una regla cambia, no se borra: se agrega una fila acá que dice qué cambió y por qué.

| Versión | Fecha | Cambio |
|---|---|---|
| 0.1 | 8/10/2026 | Primera versión, a partir de las decisiones del equipo del 7/10/2026. |
| 0.2 | 8/10/2026 | Incorpora las decisiones que entraron con el contrato de Clima. El aeropuerto es Córdoba (2.8, RN-01). La aptitud es una sola y cierra todas las pistas (6.7, RN-15, CA-10). Los arribos desviados van al alternativo más cercano disponible (2.9, RN-18, RN-25, RN-26, F-20). Si Clima no tiene datos, decide el operador (6.19, RN-24, F-21). El clima forzado no sale hacia otros grupos (6.18, F-12). Se cierra el pendiente 13 y se abren el 15, el 16 y el 17. |
