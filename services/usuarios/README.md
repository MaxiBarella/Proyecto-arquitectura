# Servicio Usuarios

Inicio de sesión de aerolíneas y operadores. Verifica el usuario y la contraseña y entrega un JWT que el gateway verifica sin volver a llamar a Usuarios (15.2 a 15.6). El contrato es [openapi.yaml](openapi.yaml).

Organización interna: **capas** (decisión 7.5 de [docs/decisiones.md](../../docs/decisiones.md)).

| Ruta | Para qué |
|---|---|
| `POST /login` | Inicio de sesión. Por el gateway, `/api/usuarios/login`. |
| `GET /health` | Para los chequeos de estado. |

**Cuentas.** No hay registro: al arrancar, el servicio carga las cuentas de prueba con las contraseñas del `.env`, hasheadas con bcrypt (15.14, 15.15). Si la cuenta ya existe, le actualiza la contraseña, así que alcanza con cambiar el `.env` y reiniciar.

| Usuario | Rol | Aerolínea | Contraseña |
|---|---|---|---|
| `operador` | `operator` | | `USUARIOS_OPERADOR_PASSWORD` |
| `aerolineas` | `airline` | `AR` | `USUARIOS_AEROLINEAS_PASSWORD` |
| `latam` | `airline` | `LA` | `USUARIOS_LATAM_PASSWORD` |

| Variable | Qué es |
|---|---|
| `MYSQL_DSN` | Conexión a MySQL, con el usuario propio de la base `usuarios` (15.16). Obligatoria. |
| `JWT_SECRET` | Clave con la que se firman los tokens. NGINX verifica con la misma (15.17). Obligatoria. |
| `USUARIOS_OPERADOR_PASSWORD`, `USUARIOS_AEROLINEAS_PASSWORD`, `USUARIOS_LATAM_PASSWORD` | Contraseñas de las cuentas de prueba. Obligatorias. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Dirección de Grafana para logs, métricas y trazas. Si falta, solo escribe los logs. |
| `PORT` | Puerto. Por defecto `8080`. |

Puerto en el arranque local: `8084`.

```sh
curl -X POST -H "content-type: application/json" \
  -d '{"username":"aerolineas","password":"<USUARIOS_AEROLINEAS_PASSWORD>"}' \
  http://localhost:8084/login
```

Tests: `go test ./...` desde esta carpeta. Los de integración levantan MySQL con Docker (testcontainers-go); `go test -short ./...` los saltea.

La tabla se crea con [db/001_schema.sql](db/001_schema.sql) la primera vez que arranca la base. Si cambia, hay que borrar el volumen: `docker compose down -v`.

| Carpeta | Para qué es |
|---|---|
| `cmd/api` | Punto de entrada: lee la configuración, carga las cuentas y pone a escuchar el servidor. |
| `internal/controllers` | Recibe `POST /login` y arma las respuestas y los errores. |
| `internal/services` | Contiene la lógica: verifica la contraseña y firma el token. |
| `internal/repositories` | Lee y guarda los usuarios en MySQL, con GORM. |
| `internal/models` | Define el usuario y la sesión. |
| `internal/observability` | Logs, métricas y trazas, copiada de Clima (ver su README). |
