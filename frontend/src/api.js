// Los pedidos a la API, todos a través del gateway (15.7).
//
// Uso:
//   const vuelos = await pedir('/vuelos/flights?status=scheduled');
//   const vuelo  = await pedir('/vuelos/flights', { metodo: 'POST', cuerpo: { ... } });
//
// - Le agrega /api adelante y el token de la sesión, si hay.
// - Devuelve el JSON de la respuesta, o null si no tiene cuerpo.
// - Si la respuesta es un error, lanza un ErrorApi con el código del error (Problem Details),
//   por ejemplo SLOT_TAKEN, para que la pantalla muestre el mensaje que corresponda.
// - Si el gateway responde 401 (token vencido o inválido, 15.18), cierra la sesión: la
//   pantalla protegida manda al usuario a iniciar sesión de nuevo.

import { cerrarSesion, sesionActual } from './sesion.js';

export class ErrorApi extends Error {
  constructor(estado, problema) {
    super(problema?.detail || problema?.title || `La API respondió ${estado}`);
    this.estado = estado;
    this.codigo = problema?.code || null;
    this.problema = problema;
  }
}

export async function pedir(ruta, { metodo = 'GET', cuerpo, encabezados = {} } = {}) {
  const sesion = sesionActual();
  const opciones = {
    method: metodo,
    headers: { Accept: 'application/json', ...encabezados },
  };
  if (sesion) {
    opciones.headers.Authorization = `Bearer ${sesion.token}`;
  }
  if (cuerpo !== undefined) {
    opciones.headers['Content-Type'] = 'application/json';
    opciones.body = JSON.stringify(cuerpo);
  }

  let respuesta;
  try {
    respuesta = await fetch(`/api${ruta}`, opciones);
  } catch {
    throw new ErrorApi(0, { title: 'No hay conexión con el sistema', code: 'NETWORK_ERROR' });
  }

  const tipo = respuesta.headers.get('Content-Type') || '';
  const datos = tipo.includes('json') ? await respuesta.json().catch(() => null) : null;

  if (!respuesta.ok) {
    if (respuesta.status === 401 && sesion) {
      cerrarSesion();
    }
    throw new ErrorApi(respuesta.status, datos);
  }
  return datos;
}
