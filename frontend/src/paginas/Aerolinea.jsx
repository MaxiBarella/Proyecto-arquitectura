import { useEffect, useState } from 'react';
import { pedir } from '../api.js';
import Aviso from '../componentes/Aviso.jsx';
import EstadoVuelo from '../componentes/EstadoVuelo.jsx';
import { useSesion } from '../sesion.js';

// Pantalla de la aerolínea: publicar, pedir slot, cancelar y ver sus vuelos. La arma Maxi
// (22.1). Solo entra el rol airline (SoloRol en App.jsx).
//
// Por ahora es un ejemplo de cómo usar la base: pide la lista de vuelos con pedir() y la
// muestra con las clases comunes. Se reemplaza entera.
export default function Aerolinea() {
  const sesion = useSesion();
  const [vuelos, setVuelos] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    pedir('/vuelos/flights')
      .then((respuesta) => setVuelos(respuesta.items))
      .catch(setError);
  }, []);

  return (
    <section>
      <h1>Vuelos de {sesion.aerolinea}</h1>
      <Aviso tipo="info">La pantalla de la aerolínea la arma Maxi (22.1). Esto es un ejemplo.</Aviso>
      <Aviso error={error} />
      {vuelos && vuelos.length === 0 && <p>Todavía no hay vuelos publicados.</p>}
      {vuelos && vuelos.length > 0 && (
        <table className="tabla">
          <thead>
            <tr>
              <th>Vuelo</th>
              <th>Tipo</th>
              <th>Horario</th>
              <th>Estado</th>
            </tr>
          </thead>
          <tbody>
            {vuelos.map((vuelo) => (
              <tr key={vuelo.id}>
                <td>{vuelo.flight_number}</td>
                <td>{vuelo.type === 'arrival' ? 'Arribo' : 'Despegue'}</td>
                <td>{new Date(vuelo.slot?.block_start || vuelo.scheduled_at).toLocaleString('es-AR')}</td>
                <td><EstadoVuelo estado={vuelo.status} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
