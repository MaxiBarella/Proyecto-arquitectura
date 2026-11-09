import { NavLink, Outlet, useNavigate } from 'react-router';
import { cerrarSesion, useSesion } from '../sesion.js';

// El marco común de todas las pantallas (22.3): el encabezado con el menú y, abajo,
// la pantalla que corresponda a la ruta.
export default function Plantilla() {
  const sesion = useSesion();
  const navegar = useNavigate();

  function salir() {
    cerrarSesion();
    navegar('/');
  }

  return (
    <>
      <header className="encabezado">
        <div className="contenedor encabezado-fila">
          <span className="marca">Aeropuerto SACO</span>
          <nav className="menu">
            <NavLink to="/" end>Tablero</NavLink>
            {sesion?.rol === 'airline' && <NavLink to="/aerolinea">Mis vuelos</NavLink>}
            {sesion?.rol === 'operator' && <NavLink to="/operador">Operación</NavLink>}
          </nav>
          <div className="usuario">
            {sesion ? (
              <>
                <span>{sesion.rol === 'airline' ? `Aerolínea ${sesion.aerolinea}` : 'Operador'}</span>
                <button type="button" className="boton boton-secundario" onClick={salir}>
                  Salir
                </button>
              </>
            ) : (
              <NavLink to="/ingreso" className="boton">Ingresar</NavLink>
            )}
          </div>
        </div>
      </header>
      <main className="contenedor contenido">
        <Outlet />
      </main>
    </>
  );
}
