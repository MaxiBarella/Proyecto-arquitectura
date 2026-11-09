import { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router';
import Aviso from '../componentes/Aviso.jsx';
import { pedir } from '../api.js';
import { iniciarSesion, iniciarSesionConToken, sesionActual, useSesion } from '../sesion.js';

// La pantalla de cada rol, adonde se va después de ingresar si no se venía de otra.
const INICIO = { airline: '/aerolinea', operator: '/operador' };

// Pantalla de ingreso (22.1, F-01). Manda el usuario y la contraseña a
// POST /api/usuarios/login (contrato de Usuarios, 15.10) y guarda la sesión con la respuesta.
export default function Ingreso() {
  const sesion = useSesion();
  const navegar = useNavigate();
  const ubicacion = useLocation();
  const [usuario, setUsuario] = useState('');
  const [contrasena, setContrasena] = useState('');
  const [error, setError] = useState(null);
  const [enviando, setEnviando] = useState(false);

  function entrar(rol) {
    navegar(ubicacion.state?.volverA || INICIO[rol] || '/', { replace: true });
  }

  async function ingresar(evento) {
    evento.preventDefault();
    setError(null);
    setEnviando(true);
    try {
      const respuesta = await pedir('/usuarios/login', {
        metodo: 'POST',
        cuerpo: { username: usuario.trim(), password: contrasena },
      });
      iniciarSesion(respuesta);
      entrar(respuesta.role);
    } catch (e) {
      // El mismo mensaje si el usuario no existe o la contraseña está mal (15.12).
      if (e.codigo === 'INVALID_CREDENTIALS') {
        e.message = 'Usuario o contraseña incorrectos.';
      } else if (e.estado >= 500) {
        e.message = 'No se pudo iniciar sesión. Probá de nuevo en un momento.';
      }
      setContrasena('');
      setError(e);
    } finally {
      setEnviando(false);
    }
  }

  if (sesion) {
    return (
      <section className="tarjeta angosta">
        <h1>Ya ingresaste</h1>
        <p>
          Estás como {sesion.rol === 'airline' ? `aerolínea ${sesion.aerolinea}` : 'operador'}.
        </p>
        <div className="fila">
          <Link to={INICIO[sesion.rol] || '/'} className="boton">Continuar</Link>
          <Link to="/" className="boton boton-secundario">Ver el tablero</Link>
        </div>
      </section>
    );
  }

  return (
    <section className="tarjeta angosta">
      <h1>Ingresar</h1>
      <p className="ayuda">
        Para aerolíneas y operadores del aeropuerto. El tablero se puede ver sin ingresar.
      </p>

      <form onSubmit={ingresar} className="formulario">
        <label className="campo">
          Usuario
          <input
            value={usuario}
            onChange={(e) => setUsuario(e.target.value)}
            autoComplete="username"
            autoFocus
            required
          />
        </label>
        <label className="campo">
          Contraseña
          <input
            type="password"
            value={contrasena}
            onChange={(e) => setContrasena(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        <Aviso error={error} />
        <button type="submit" className="boton" disabled={enviando}>
          {enviando ? 'Ingresando…' : 'Ingresar'}
        </button>
      </form>

      {import.meta.env.DEV && <IngresoConToken alEntrar={entrar} />}
    </section>
  );
}

// Solo en desarrollo (npm run dev): entrar con un token de nginx/token-de-prueba.sh (15.20),
// sin pasar por Usuarios. No aparece en lo que sirve NGINX.
function IngresoConToken({ alEntrar }) {
  const [token, setToken] = useState('');
  const [error, setError] = useState(null);

  function entrarConToken(evento) {
    evento.preventDefault();
    try {
      iniciarSesionConToken(token.trim());
    } catch {
      setError(new Error('Eso no parece un token.'));
      return;
    }
    alEntrar(sesionActual()?.rol);
  }

  return (
    <details>
      <summary className="ayuda">Probar con un token (solo desarrollo)</summary>
      <form onSubmit={entrarConToken} className="formulario">
        <p className="ayuda">
          Generalo con <code>nginx/token-de-prueba.sh airline AR</code> o{' '}
          <code>nginx/token-de-prueba.sh operator</code>.
        </p>
        <label className="campo">
          Token
          <textarea value={token} onChange={(e) => setToken(e.target.value)} rows={4} required />
        </label>
        <Aviso error={error} />
        <button type="submit" className="boton boton-secundario">Entrar con el token</button>
      </form>
    </details>
  );
}
