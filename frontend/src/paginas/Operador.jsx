import { useCallback, useEffect, useState } from 'react';
import { pedir } from '../api.js';
import Aviso from '../componentes/Aviso.jsx';

// Pantalla del operador (22.1): ver las pistas y el clima (F-21), cerrar y reabrir pistas
// (F-10, F-11), dar de alta una pista (F-09), forzar el clima (F-12) y marcar aeropuertos
// alternativos como no disponibles (F-20). Solo entra el rol operator (SoloRol en App.jsx);
// Pistas y Clima controlan además el rol (CA-19).

// Las rutas de Clima para forzar una condición (9.13). Pasan por el gateway con el rol operator.
const FORZADO = '/clima/internal/airports/SACO/forced-conditions';

// Clima completa los valores que no se mandan con el último dato real (9.12). Por eso cada
// condición manda todos los que deciden la aptitud: si no, un dato real malo seguiría mandando.
const CONDICIONES = {
  tormenta: { thunderstorm: true, wind_speed_kmh: 90, wind_gust_kmh: 110, visibility_m: 3000, precipitation_mm: 20 },
  bueno: { thunderstorm: false, wind_speed_kmh: 10, wind_gust_kmh: 15, visibility_m: 10000, precipitation_mm: 0 },
};

// Cada cuánto se vuelve a pedir el estado: el clima cierra y reabre las pistas solo.
const REFRESCO_MS = 10000;

// Mensajes para los códigos de error del contrato de Pistas.
const MENSAJES = {
  RUNWAY_CLOSED_BY_WEATHER:
    'La cerró el clima y Clima tiene datos: se reabre sola cuando mejore. Para probar, forzá buen tiempo.',
  RUNWAY_ALREADY_EXISTS: 'Ya hay una pista con ese nombre.',
  RUNWAY_NOT_FOUND: 'Esa pista no existe.',
  ALTERNATE_NOT_FOUND: 'Ese aeropuerto no está en la lista de alternativos.',
  UNAVAILABLE: 'Pistas no responde. Probá de nuevo en un momento.',
};

function conMensaje(e) {
  if (MENSAJES[e.codigo]) e.message = MENSAJES[e.codigo];
  return e;
}

export default function Operador() {
  const [estado, setEstado] = useState(null);
  const [alternativos, setAlternativos] = useState(null);
  const [error, setError] = useState(null);
  const [aviso, setAviso] = useState(null);

  const cargar = useCallback(async () => {
    try {
      const [pistas, alts] = await Promise.all([pedir('/pistas/runways'), pedir('/pistas/alternate-airports')]);
      setEstado(pistas);
      setAlternativos(alts.alternate_airports);
      setError(null);
    } catch (e) {
      setError(conMensaje(e));
    }
  }, []);

  useEffect(() => {
    cargar();
    const id = setInterval(cargar, REFRESCO_MS);
    return () => clearInterval(id);
  }, [cargar]);

  // Después de forzar el clima, Pistas se entera por un evento (4.2): se vuelve a pedir el
  // estado un par de veces para que se vea sin esperar el refresco.
  function refrescar() {
    cargar();
    setTimeout(cargar, 1500);
    setTimeout(cargar, 4000);
  }

  // Corre una acción y muestra el resultado o el error.
  async function hacer(accion, mensajeOk) {
    setAviso(null);
    try {
      await accion();
      setAviso({ tipo: 'ok', texto: mensajeOk });
      refrescar();
    } catch (e) {
      setAviso({ tipo: 'error', error: conMensaje(e) });
    }
  }

  return (
    <section>
      <h1>Operación</h1>
      <Aviso error={error} />
      {aviso && (aviso.tipo === 'ok' ? <Aviso tipo="ok">{aviso.texto}</Aviso> : <Aviso error={aviso.error} />)}

      {estado && <Clima clima={estado.weather} hacer={hacer} />}
      {estado && <Pistas pistas={estado.runways} conDatos={estado.weather.data_available} hacer={hacer} />}
      <AltaDePista hacer={hacer} />
      {alternativos && <Alternativos lista={alternativos} hacer={hacer} />}
    </section>
  );
}

// El estado del clima, el aviso de "Clima sin datos" (F-21, RN-24) y el forzado (F-12).
function Clima({ clima, hacer }) {
  let texto = 'Todavía no llegó ningún dato de Clima.';
  if (!clima.data_available) {
    texto = 'Sin datos.';
  } else if (clima.operable === true) {
    texto = 'Se puede operar.';
  } else if (clima.operable === false) {
    texto = 'No se puede operar: las pistas se cerraron solas.';
  }

  function forzar(nombre, mensaje) {
    hacer(() => pedir(FORZADO, { metodo: 'PUT', cuerpo: CONDICIONES[nombre] }), mensaje);
  }

  return (
    <div className="tarjeta">
      <h2>Clima</h2>
      {!clima.data_available && (
        <Aviso tipo="error">
          Clima no puede obtener los datos. Las pistas no se cierran ni se reabren solas: decidí a mano.
        </Aviso>
      )}
      <p>{texto}</p>
      <p className="ayuda">
        Para probar el cierre de pistas, forzá una condición. Manda sobre el dato real hasta que la quites, y
        no cambia lo que Clima publica para otros grupos.
      </p>
      <div className="fila">
        <button type="button" className="boton boton-peligro" onClick={() => forzar('tormenta', 'Tormenta forzada.')}>
          Forzar tormenta
        </button>
        <button type="button" className="boton" onClick={() => forzar('bueno', 'Buen tiempo forzado.')}>
          Forzar buen tiempo
        </button>
        <button
          type="button"
          className="boton boton-secundario"
          onClick={() => hacer(() => pedir(FORZADO, { metodo: 'DELETE' }), 'Condición forzada quitada: vuelve el dato real.')}
        >
          Quitar la condición forzada
        </button>
      </div>
    </div>
  );
}

const MOTIVOS = { weather: 'Cerrada por el clima', operator: 'Cerrada por el operador' };

// Las pistas, con cerrar y reabrir (F-10, F-11).
function Pistas({ pistas, conDatos, hacer }) {
  function cerrar(id) {
    const seguro = window.confirm(
      `¿Cerrar la pista ${id}? Los vuelos con slot en ella que todavía no empezaron lo pierden: los despegues quedan demorados y los arribos se desvían.`,
    );
    if (seguro) {
      hacer(() => pedir(`/pistas/runways/${id}/close`, { metodo: 'POST' }), `Pista ${id} cerrada.`);
    }
  }

  return (
    <div className="tarjeta">
      <h2>Pistas</h2>
      <table className="tabla">
        <thead>
          <tr>
            <th>Pista</th>
            <th>Estado</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {pistas.map((p) => (
            <tr key={p.id}>
              <td>{p.id}</td>
              <td>{p.status === 'open' ? 'Abierta' : MOTIVOS[p.closed_reason]}</td>
              <td>
                {p.status === 'open' && (
                  <button type="button" className="boton boton-peligro" onClick={() => cerrar(p.id)}>
                    Cerrar
                  </button>
                )}
                {p.status === 'closed' && (p.closed_reason === 'operator' || !conDatos) && (
                  <button
                    type="button"
                    className="boton"
                    onClick={() => hacer(() => pedir(`/pistas/runways/${p.id}/reopen`, { metodo: 'POST' }), `Pista ${p.id} reabierta.`)}
                  >
                    Reabrir
                  </button>
                )}
                {p.status === 'closed' && p.closed_reason === 'weather' && conDatos && (
                  <span className="ayuda">Se reabre sola cuando mejore el clima.</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Alta de una pista (F-09).
function AltaDePista({ hacer }) {
  const [id, setId] = useState('');

  function enviar(evento) {
    evento.preventDefault();
    const nombre = id.trim();
    hacer(async () => {
      await pedir('/pistas/runways', { metodo: 'POST', cuerpo: { id: nombre } });
      setId('');
    }, `Pista ${nombre} dada de alta.`);
  }

  return (
    <div className="tarjeta">
      <h2>Dar de alta una pista</h2>
      <form onSubmit={enviar} className="formulario">
        <label className="campo">
          Nombre
          <input value={id} onChange={(e) => setId(e.target.value)} placeholder="09-27" required />
        </label>
        <p className="ayuda">Letras, números y guiones, sin barra, como 05-23.</p>
        <button type="submit" className="boton">Dar de alta</button>
      </form>
    </div>
  );
}

// Los aeropuertos alternativos, del más cercano al más lejano (RN-25, F-20).
function Alternativos({ lista, hacer }) {
  function cambiar(a) {
    const accion = a.available ? 'disable' : 'enable';
    hacer(
      () => pedir(`/pistas/alternate-airports/${a.icao}/${accion}`, { metodo: 'POST' }),
      `${a.name} ${a.available ? 'ya no está disponible' : 'vuelve a estar disponible'}.`,
    );
  }

  return (
    <div className="tarjeta">
      <h2>Aeropuertos alternativos</h2>
      <p className="ayuda">Un arribo desviado va al primero disponible de la lista.</p>
      <table className="tabla">
        <thead>
          <tr>
            <th>Aeropuerto</th>
            <th>Código</th>
            <th>Estado</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {lista.map((a) => (
            <tr key={a.icao}>
              <td>{a.name}</td>
              <td>{a.icao}</td>
              <td>{a.available ? 'Disponible' : 'No disponible'}</td>
              <td>
                <button type="button" className="boton boton-secundario" onClick={() => cambiar(a)}>
                  {a.available ? 'Marcar no disponible' : 'Volver a habilitar'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
