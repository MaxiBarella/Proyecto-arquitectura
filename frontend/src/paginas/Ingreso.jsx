import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router';
import Aviso from '../componentes/Aviso.jsx';
import { iniciarSesionConToken } from '../sesion.js';

// Pantalla de ingreso. La arma Male (22.1) contra POST /api/usuarios/login, y al recibir la
// respuesta llama a iniciarSesion(respuesta) de sesion.js.
//
// Mientras tanto tiene un formulario para pegar un token de nginx/token-de-prueba.sh (15.20),
// así se pueden probar las pantallas de la aerolínea y del operador. Se saca cuando ande el
// ingreso de verdad.
export default function Ingreso() {
  const [token, setToken] = useState('');
  const [error, setError] = useState(null);
  const navegar = useNavigate();
  const ubicacion = useLocation();

  function entrarConToken(evento) {
    evento.preventDefault();
    try {
      iniciarSesionConToken(token.trim());
    } catch {
      setError(new Error('Eso no parece un token.'));
      return;
    }
    navegar(ubicacion.state?.volverA || '/');
  }

  return (
    <section className="tarjeta angosta">
      <h1>Ingresar</h1>
      <Aviso tipo="info">La pantalla de ingreso la arma Male (22.1).</Aviso>

      <form onSubmit={entrarConToken} className="formulario">
        <h2>Probar con un token</h2>
        <p className="ayuda">
          Solo para desarrollo. Generalo con <code>nginx/token-de-prueba.sh airline AR</code> o{' '}
          <code>nginx/token-de-prueba.sh operator</code>.
        </p>
        <label className="campo">
          Token
          <textarea value={token} onChange={(e) => setToken(e.target.value)} rows={4} required />
        </label>
        <Aviso error={error} />
        <button type="submit" className="boton">Entrar</button>
      </form>
    </section>
  );
}
