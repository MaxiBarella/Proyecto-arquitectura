import { useCallback, useEffect, useState } from 'react';
import { pedir } from '../api.js';
import Aviso from '../componentes/Aviso.jsx';
import EstadoVuelo from '../componentes/EstadoVuelo.jsx';
import { useSesion } from '../sesion.js';
import { AMBITOS, BLOQUES, TIPOS, aUTC, horaCorta, hoy } from '../horarios.js';

// Pantalla de la aerolínea (22.1): publicar vuelos (F-02), ver los suyos (F-07), pedir slot
// (F-03, F-05) y cancelar (F-06). Solo entra el rol airline (SoloRol en App.jsx); el
// gateway y Vuelos controlan además que cada aerolínea opere solo sus vuelos (RN-30, RN-31).

// Nuestro aeropuerto (RN-01): un despegue sale de acá y un arribo llega acá.
const CORDOBA = 'Córdoba';

// Las pistas con las que arranca el sistema (RN-02). Son una sugerencia: se puede escribir otra.
const PISTAS = ['05-23', '18-36'];

// Vuelos (RN-07, RN-12) pide slot de pendiente o, si es un despegue, de demorado.
function puedePedirSlot(v) {
  return v.status === 'pending_slot' || (v.status === 'delayed' && v.type === 'departure');
}

// Se cancela si no está en un estado final (RN-21).
function puedeCancelar(v) {
  return ['pending_slot', 'scheduled', 'delayed'].includes(v.status);
}

// Mensajes para los códigos de error del contrato de Vuelos.
const MENSAJES = {
  SLOT_TAKEN: 'Ese slot ya está ocupado. Estos son los libres más cercanos:',
  RUNWAY_CLOSED: 'La pista está cerrada. Elegí otra.',
  RUNWAY_NOT_FOUND: 'Esa pista no existe.',
  RUNWAYS_UNAVAILABLE: 'No se pudo pedir el slot porque Pistas no responde. El vuelo sigue como estaba; probá de nuevo en un momento.',
  INVALID_TRANSITION: 'El vuelo ya no está en un estado que permita esa acción.',
};

function conMensaje(e) {
  if (MENSAJES[e.codigo]) e.message = MENSAJES[e.codigo];
  return e;
}

export default function Aerolinea() {
  const sesion = useSesion();
  const [estado, setEstado] = useState('');
  const [pagina, setPagina] = useState(1);
  const [lista, setLista] = useState(null);
  const [error, setError] = useState(null);
  const [aviso, setAviso] = useState(null);

  const cargar = useCallback(async () => {
    const consulta = new URLSearchParams({ page: String(pagina) });
    if (estado) consulta.set('status', estado);
    try {
      setLista(await pedir(`/vuelos/flights?${consulta}`));
      setError(null);
    } catch (e) {
      setError(e);
    }
  }, [estado, pagina]);

  useEffect(() => {
    cargar();
  }, [cargar]);

  // Después de un cambio: el slot asignado llega con un evento de Pistas (4.1), así que se
  // vuelve a pedir la lista un par de veces para que se vea sin recargar.
  function refrescar() {
    cargar();
    setTimeout(cargar, 1500);
    setTimeout(cargar, 4000);
  }

  const paginas = lista ? Math.max(1, Math.ceil(lista.total / lista.page_size)) : 1;

  return (
    <section>
      <h1>Vuelos de {sesion.aerolinea}</h1>
      <p className="ayuda">Los horarios están en hora de Argentina.</p>

      <Publicar
        aerolinea={sesion.aerolinea}
        alPublicar={(v) => {
          setAviso(`Vuelo ${v.flight_number} publicado. Queda pendiente de slot hasta que pidas uno.`);
          refrescar();
        }}
      />

      <h2>Mis vuelos</h2>
      <Aviso tipo="ok">{aviso}</Aviso>
      <Aviso error={error} />
      <div className="fila">
        <label className="campo">
          Estado
          <select value={estado} onChange={(e) => { setEstado(e.target.value); setPagina(1); }}>
            <option value="">Todos</option>
            <option value="pending_slot">Pendiente de slot</option>
            <option value="scheduled">Programado</option>
            <option value="delayed">Demorado</option>
            <option value="completed">Finalizado</option>
            <option value="diverted">Desviado</option>
            <option value="cancelled">Cancelado</option>
          </select>
        </label>
        <button type="button" className="boton boton-secundario" onClick={cargar}>Actualizar</button>
      </div>

      {lista && lista.items.length === 0 && <p>No hay vuelos{estado ? ' en ese estado' : ' publicados'}.</p>}
      {lista && lista.items.length > 0 && (
        <div style={{ overflowX: 'auto' }}>
          <table className="tabla">
          <thead>
            <tr>
              <th>Vuelo</th>
              <th>Tipo</th>
              <th>Origen</th>
              <th>Destino</th>
              <th>Publicado</th>
              <th>Slot</th>
              <th>Estado</th>
              <th>Acciones</th>
            </tr>
          </thead>
          <tbody>
            {lista.items.map((v) => (
              <FilaVuelo
                key={v.id}
                vuelo={v}
                alCambiar={(mensaje) => {
                  setAviso(mensaje);
                  refrescar();
                }}
              />
            ))}
          </tbody>
        </table>
          </div>
      )}
      {lista && paginas > 1 && (
        <div className="fila">
          <button type="button" className="boton boton-secundario" disabled={pagina <= 1} onClick={() => setPagina(pagina - 1)}>
            Anterior
          </button>
          <span>Página {pagina} de {paginas}</span>
          <button type="button" className="boton boton-secundario" disabled={pagina >= paginas} onClick={() => setPagina(pagina + 1)}>
            Siguiente
          </button>
        </div>
      )}
    </section>
  );
}

// Publicar un vuelo (F-02). El vuelo queda pendiente de slot (RN-06).
function Publicar({ aerolinea, alPublicar }) {
  const vacio = { numero: aerolinea || '', tipo: 'departure', ambito: 'domestic', ciudad: '', fecha: hoy(), hora: '' };
  const [datos, setDatos] = useState(vacio);
  const [error, setError] = useState(null);
  const [enviando, setEnviando] = useState(false);

  const despegue = datos.tipo === 'departure';
  const cambiar = (campo) => (e) => setDatos({ ...datos, [campo]: e.target.value });

  async function publicar(evento) {
    evento.preventDefault();
    setError(null);
    setEnviando(true);
    try {
      const vuelo = await pedir('/vuelos/flights', {
        metodo: 'POST',
        cuerpo: {
          flight_number: datos.numero.trim().toUpperCase(),
          type: datos.tipo,
          scope: datos.ambito,
          origin: despegue ? CORDOBA : datos.ciudad.trim(),
          destination: despegue ? datos.ciudad.trim() : CORDOBA,
          scheduled_at: aUTC(datos.fecha, datos.hora),
        },
      });
      setDatos(vacio);
      alPublicar(vuelo);
    } catch (e) {
      setError(conMensaje(e));
    } finally {
      setEnviando(false);
    }
  }

  return (
    <form className="formulario tarjeta" onSubmit={publicar}>
      <h2>Publicar un vuelo</h2>
      <div className="fila">
        <label className="campo">
          Número
          <input value={datos.numero} onChange={cambiar('numero')} placeholder="AR1550" required pattern="[A-Za-z0-9]{2}[0-9]{1,4}" size={8} />
        </label>
        <label className="campo">
          Tipo
          <select value={datos.tipo} onChange={cambiar('tipo')}>
            {Object.entries(TIPOS).map(([v, n]) => <option key={v} value={v}>{n}</option>)}
          </select>
        </label>
        <label className="campo">
          Ámbito
          <select value={datos.ambito} onChange={cambiar('ambito')}>
            {Object.entries(AMBITOS).map(([v, n]) => <option key={v} value={v}>{n}</option>)}
          </select>
        </label>
        <label className="campo">
          {despegue ? 'Destino' : 'Origen'}
          <input value={datos.ciudad} onChange={cambiar('ciudad')} placeholder="Buenos Aires" required />
        </label>
        <label className="campo">
          Fecha
          <input type="date" value={datos.fecha} min={hoy()} onChange={cambiar('fecha')} required />
        </label>
        <label className="campo">
          Hora
          <input type="time" value={datos.hora} onChange={cambiar('hora')} required />
        </label>
      </div>
      <p className="ayuda">{despegue ? `Sale de ${CORDOBA}.` : `Llega a ${CORDOBA}.`}</p>
      <Aviso error={error} />
      <button type="submit" className="boton" disabled={enviando}>{enviando ? 'Publicando…' : 'Publicar'}</button>
    </form>
  );
}

// Un vuelo de la lista, con sus acciones.
function FilaVuelo({ vuelo, alCambiar }) {
  const [pidiendo, setPidiendo] = useState(false);
  const [error, setError] = useState(null);

  async function cancelar() {
    if (!window.confirm(`¿Cancelar el vuelo ${vuelo.flight_number}? No se puede deshacer.`)) return;
    setError(null);
    try {
      await pedir(`/vuelos/flights/${vuelo.id}/cancel`, { metodo: 'POST' });
      alCambiar(`Vuelo ${vuelo.flight_number} cancelado.`);
    } catch (e) {
      setError(conMensaje(e));
    }
  }

  return (
    <>
      <tr>
        <td><strong>{vuelo.flight_number}</strong></td>
        <td>{TIPOS[vuelo.type]}</td>
        <td>{vuelo.origin}</td>
        <td>{vuelo.destination}</td>
        <td>{horaCorta(vuelo.scheduled_at)}</td>
        <td>{vuelo.slot ? `${vuelo.slot.runway_id} · ${horaCorta(vuelo.slot.block_start)}` : '—'}</td>
        <td>
          <EstadoVuelo estado={vuelo.status} />
          {vuelo.diverted_to && <span className="ayuda"> a {vuelo.diverted_to}</span>}
        </td>
        <td>
          <div className="fila">
            {puedePedirSlot(vuelo) && (
              <button type="button" className="boton" onClick={() => setPidiendo(!pidiendo)}>
                {pidiendo ? 'Cerrar' : 'Pedir slot'}
              </button>
            )}
            {puedeCancelar(vuelo) && (
              <button type="button" className="boton boton-peligro" onClick={cancelar}>Cancelar</button>
            )}
          </div>
        </td>
      </tr>
      {(pidiendo || error) && (
        <tr>
          <td colSpan={8}>
            <Aviso error={error} />
            {pidiendo && (
              <PedirSlot
                vuelo={vuelo}
                alAsignar={(slot) => {
                  setPidiendo(false);
                  alCambiar(`Slot pedido para ${vuelo.flight_number}: pista ${slot.runway_id} a las ${horaCorta(slot.block_start)}. En unos segundos pasa a programado.`);
                }}
              />
            )}
          </td>
        </tr>
      )}
    </>
  );
}

// Pedir un slot: una pista y un bloque de 15 minutos (RN-03, RN-04). Si está ocupado,
// muestra los 3 libres más cercanos para pedir uno con un clic (RN-09).
function PedirSlot({ vuelo, alAsignar }) {
  const inicial = vuelo.scheduled_at;
  const [pista, setPista] = useState(PISTAS[0]);
  const [fecha, setFecha] = useState(new Date(inicial).toLocaleDateString('en-CA', { timeZone: 'America/Argentina/Cordoba' }));
  const [hora, setHora] = useState('');
  const [error, setError] = useState(null);
  const [libres, setLibres] = useState([]);
  const [enviando, setEnviando] = useState(false);

  async function pedirSlot(slot) {
    setError(null);
    setLibres([]);
    setEnviando(true);
    try {
      const respuesta = await pedir(`/vuelos/flights/${vuelo.id}/slot-request`, { metodo: 'POST', cuerpo: slot });
      alAsignar(respuesta.slot);
    } catch (e) {
      setError(conMensaje(e));
      setLibres(e.problema?.nearest_free_slots || []);
    } finally {
      setEnviando(false);
    }
  }

  return (
    <form
      className="formulario"
      onSubmit={(e) => {
        e.preventDefault();
        pedirSlot({ runway_id: pista.trim(), block_start: aUTC(fecha, hora) });
      }}
    >
      <div className="fila">
        <label className="campo">
          Pista
          <input list="pistas" value={pista} onChange={(e) => setPista(e.target.value)} required size={6} />
          <datalist id="pistas">{PISTAS.map((p) => <option key={p} value={p} />)}</datalist>
        </label>
        <label className="campo">
          Fecha
          <input type="date" value={fecha} onChange={(e) => setFecha(e.target.value)} required />
        </label>
        <label className="campo">
          Bloque
          <select value={hora} onChange={(e) => setHora(e.target.value)} required>
            <option value="">Elegí</option>
            {BLOQUES.map((b) => <option key={b} value={b}>{b}</option>)}
          </select>
        </label>
        <button type="submit" className="boton" disabled={enviando}>{enviando ? 'Pidiendo…' : 'Pedir'}</button>
      </div>
      <Aviso error={error} />
      {libres.length > 0 && (
        <div className="fila">
          {libres.map((s) => (
            <button
              key={`${s.runway_id}-${s.block_start}`}
              type="button"
              className="boton boton-secundario"
              disabled={enviando}
              onClick={() => pedirSlot(s)}
            >
              Pista {s.runway_id} · {horaCorta(s.block_start)}
            </button>
          ))}
        </div>
      )}
    </form>
  );
}
