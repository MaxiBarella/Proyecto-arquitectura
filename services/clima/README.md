# Servicio Clima

Organización interna: **capas** (decisión 7.3 de [docs/decisiones.md](../../docs/decisiones.md)).

Es la estructura inicial: el servicio arranca y responde `GET /health`. Todavía no tiene lógica.

| Carpeta | Para qué es |
|---|---|
| `cmd/api` | Punto de entrada: arma el servidor y lo pone a escuchar. |
| `internal/controllers` | Recibe los pedidos HTTP y arma las respuestas. |
| `internal/services` | Contiene la lógica: condiciones actuales y aptitud operativa. |
| `internal/repositories` | Lee y guarda las observaciones en MongoDB. |
| `internal/models` | Define los datos que maneja el servicio. |

Puerto en el arranque local: `8083`.
