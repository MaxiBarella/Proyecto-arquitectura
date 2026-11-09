# Frontend

La interfaz del sistema, en React con Vite (22.2) y JavaScript (22.5). NGINX la compila al construir su imagen y la sirve en `http://localhost:8080`, desde el mismo lugar que la API (15.7).

## Cómo trabajar en tu pantalla

Cada uno tiene su archivo en `src/paginas/` (22.1):

| Pantalla | Archivo | Dirección | Quién |
|---|---|---|---|
| Tablero público | `Tablero.jsx` | `/` | Maxi |
| Aerolínea | `Aerolinea.jsx` | `/aerolinea` | Maxi |
| Operador | `Operador.jsx` | `/operador` | Carola |
| Ingreso | `Ingreso.jsx` | `/ingreso` | Male |

1. Levantá el sistema: `docker compose up --build`, desde la raíz.
2. En otra terminal, desde `frontend/`: `npm install` (la primera vez) y `npm run dev`.
3. Abrí `http://localhost:5173`. Cada vez que guardás, la pantalla se actualiza sola. Los pedidos a `/api` van al gateway del paso 1.

Para ver el resultado final, como lo sirve NGINX: `docker compose up --build nginx` y `http://localhost:8080`.

**Para entrar como aerolínea u operador:** en la pantalla de ingreso, con las cuentas de Usuarios: `aerolineas` (AR), `latam` (LA) u `operador`, con las contraseñas de tu `.env` (15.15, 15.22). Con `npm run dev` hay además un desplegable para entrar con un token de `nginx/token-de-prueba.sh airline AR` (o `operator`), sin pasar por Usuarios; no aparece en lo que sirve NGINX.

## Lo que da la base (22.3)

- **`src/api.js`:** `pedir(ruta, { metodo, cuerpo })` hace el pedido al gateway con el token de la sesión y devuelve el JSON. Si la API responde un error, lanza un `ErrorApi` con `estado` y `codigo` (por ejemplo `SLOT_TAKEN`). Si responde `401`, cierra la sesión.
  ```js
  const vuelos = await pedir('/vuelos/flights?status=scheduled');
  await pedir(`/vuelos/flights/${id}/cancel`, { metodo: 'POST' });
  ```
- **`src/sesion.js`:** `useSesion()` devuelve la sesión (`{ token, rol, aerolinea, vence }`) o `null`. `iniciarSesion(respuesta)` guarda la respuesta del login de Usuarios y `cerrarSesion()` la borra. Se guarda en `sessionStorage` (22.7).
- **`src/componentes/`:**
  - `Plantilla.jsx`: el encabezado con el menú, según el rol.
  - `SoloRol.jsx`: no deja entrar a una pantalla sin sesión o con otro rol. Las rutas protegidas están en `App.jsx`.
  - `Aviso.jsx`: un mensaje o un error de la API.
  - `EstadoVuelo.jsx`: el estado de un vuelo con su color.
- **`src/estilos.css`:** los estilos comunes (22.6). Usá sus clases en vez de estilos propios, así todas las pantallas se ven iguales. La lista está al principio del archivo.

`Aerolinea.jsx` tiene un ejemplo que usa todo esto: pide los vuelos y los muestra en una tabla.
