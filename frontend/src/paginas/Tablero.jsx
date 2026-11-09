import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';
import { pedir } from '../api.js';
import Aviso from '../componentes/Aviso.jsx';
import EstadoVuelo from '../componentes/EstadoVuelo.jsx';
import { AMBITOS, TIPOS, horaCorta } from '../horarios.js';

// Tablero público de arribos y despegues (F-13, F-14). Lee GET /api/vuelos/board, que pasa
// por el gateway sin token (15.7). Los filtros quedan en la dirección, así se puede
// recargar o compartir la búsqueda.

// Cada cuánto se vuelve a pedir el tablero solo. El tablero guarda cada consulta 60
// segundos (10.3), así que pedirlo más seguido no muestra nada nuevo.
const REFRESCO_MS = 60_000;

const ESTADOS = {
  pending_slot: 'Pendiente de slot',
  scheduled: 'Programado',
  delayed: 'Demorado',
  completed: 'Finalizado',
  diverted: 'Desviado',
  cancelled: 'Cancelado',
};

const FILTROS = ['type', 'status', 'airline', 'date', 'q', 'sort', 'page'];

export default function Tablero() {
  const [parametros, setParametros] = useSearchParams();
  const [pagina, setPagina] = useState(null);
  const [error, setError] = useState(null);
  const [cargando, setCargando] = useState(true);
  // El texto se escribe acá y se busca recién al dejar de tipear, para no pedir el
  // tablero en cada letra.
  const [texto, setTexto] = useState(parametros.get('q') || '');

  const consulta = new URLSearchParams();
  FILTROS.forEach((f) => parametros.get(f) && consulta.set(f, parametros.get(f)));
  const ruta = `/vuelos/board${consulta.size ? `?${consulta}` : ''}`;

  useEffect(() => {
    let vigente = true;
    async function cargar() {
      try {
        const respuesta = await pedir(ruta);
        if (vigente) {
          setPagina(respuesta);
          setError(null);
        }
      } catch (e) {
        if (vigente) {
          if (e.codigo === 'BOARD_UNAVAILABLE') {
            e.message = 'El tablero no está disponible en este momento. Se vuelve a intentar solo en un minuto.';
          }
          setError(e);
        }
      } finally {
        if (vigente) setCargando(false);
      }
    }
    setCargando(true);
    cargar();
    const intervalo = setInterval(cargar, REFRESCO_MS);
    return () => {
      vigente = false;
      clearInterval(intervalo);
    };
  }, [ruta]);

  useEffect(() => {
    const espera = setTimeout(() => {
      if ((parametros.get('q') || '') !== texto.trim()) cambiar('q', texto.trim());
    }, 400);
    return () => clearTimeout(espera);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [texto]);

  // Cambia un filtro y vuelve a la primera página.
  function cambiar(nombre, valor) {
    const nuevos = new URLSearchParams(parametros);
    if (valor) nuevos.set(nombre, valor);
    else nuevos.delete(nombre);
    if (nombre !== 'page') nuevos.delete('page');
    setParametros(nuevos);
  }

  const paginaActual = pagina?.page || 1;
  const paginas = pagina ? Math.max(1, Math.ceil(pagina.total / pagina.page_size)) : 1;

  return (
    <section>
      <h1>Tablero de vuelos</h1>
      <p className="ayuda">
        Arribos y despegues de Córdoba. Los horarios están en hora de Argentina. Un cambio puede
        tardar hasta 2 minutos en verse.
      </p>

      <form className="fila" onSubmit={(e) => e.preventDefault()}>
        <label className="campo">
          Buscar
          <input
            type="search"
            value={texto}
            onChange={(e) => setTexto(e.target.value)}
            placeholder="Vuelo o ciudad, por ejemplo AR15 o cordoba"
            maxLength={60}
          />
        </label>
        <label className="campo">
          Tipo
          <select value={parametros.get('type') || ''} onChange={(e) => cambiar('type', e.target.value)}>
            <option value="">Todos</option>
            {Object.entries(TIPOS).map(([v, n]) => <option key={v} value={v}>{n}</option>)}
          </select>
        </label>
        <label className="campo">
          Estado
          <select value={parametros.get('status') || ''} onChange={(e) => cambiar('status', e.target.value)}>
            <option value="">Todos</option>
            {Object.entries(ESTADOS).map(([v, n]) => <option key={v} value={v}>{n}</option>)}
          </select>
        </label>
        <label className="campo">
          Aerolínea
          <input
            value={parametros.get('airline') || ''}
            onChange={(e) => cambiar('airline', e.target.value.toUpperCase())}
            placeholder="AR"
            maxLength={3}
            size={4}
          />
        </label>
        <label className="campo">
          Fecha
          <input type="date" value={parametros.get('date') || ''} onChange={(e) => cambiar('date', e.target.value)} />
        </label>
        <label className="campo">
          Ordenar por
          <select value={parametros.get('sort') || 'scheduled_at'} onChange={(e) => cambiar('sort', e.target.value)}>
            <option value="scheduled_at">Horario</option>
            <option value="airline">Aerolínea</option>
            <option value="scope">Ámbito</option>
          </select>
        </label>
      </form>

      {!parametros.get('date') && <p className="ayuda">Sin fecha elegida se muestran los vuelos de hoy en adelante.</p>}

      <Aviso error={error} />
      {cargando && !pagina && <p>Cargando…</p>}
      {pagina && pagina.items.length === 0 && <p>No hay vuelos con esos filtros.</p>}

      {pagina && pagina.items.length > 0 && (
        <>
          <div style={{ overflowX: 'auto' }}>
          <table className="tabla">
            <thead>
              <tr>
                <th>Horario</th>
                <th>Vuelo</th>
                <th>Aerolínea</th>
                <th>Tipo</th>
                <th>Origen</th>
                <th>Destino</th>
                <th>Ámbito</th>
                <th>Pista</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {pagina.items.map((v) => (
                <tr key={v.id}>
                  {/* Con slot vale el horario del bloque; sin slot, el publicado (RN-39). */}
                  <td>{horaCorta(v.slot?.block_start || v.scheduled_at)}</td>
                  <td><strong>{v.flight_number}</strong></td>
                  <td>{v.airline}</td>
                  <td>{TIPOS[v.type] || v.type}</td>
                  <td>{v.origin}</td>
                  <td>{v.destination}</td>
                  <td>{AMBITOS[v.scope] || v.scope}</td>
                  <td>{v.slot?.runway_id || '—'}</td>
                  <td>
                    <EstadoVuelo estado={v.status} />
                    {v.diverted_to && <span className="ayuda"> a {v.diverted_to}</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>

          <div className="fila">
            <button
              type="button"
              className="boton boton-secundario"
              disabled={paginaActual <= 1}
              onClick={() => cambiar('page', String(paginaActual - 1))}
            >
              Anterior
            </button>
            <span>
              Página {paginaActual} de {paginas} · {pagina.total} vuelos
            </span>
            <button
              type="button"
              className="boton boton-secundario"
              disabled={paginaActual >= paginas}
              onClick={() => cambiar('page', String(paginaActual + 1))}
            >
              Siguiente
            </button>
          </div>
        </>
      )}
    </section>
  );
}
