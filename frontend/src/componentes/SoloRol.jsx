import { Navigate, Outlet, useLocation } from 'react-router';
import { useSesion } from '../sesion.js';

// Protege las pantallas de un rol. Sin sesión (o si venció), manda a iniciar sesión y
// después vuelve. Con otro rol, avisa que no tiene acceso.
//
// Es solo para no mostrar pantallas que no sirven: quien controla de verdad es el gateway,
// que verifica el token (15.4), y cada servicio, que decide qué puede hacer cada rol (15.6).
export default function SoloRol({ rol }) {
  const sesion = useSesion();
  const ubicacion = useLocation();

  if (!sesion) {
    return <Navigate to="/ingreso" replace state={{ volverA: ubicacion.pathname }} />;
  }
  if (sesion.rol !== rol) {
    return <p className="aviso aviso-error">Esta pantalla no es para tu rol.</p>;
  }
  return <Outlet />;
}
