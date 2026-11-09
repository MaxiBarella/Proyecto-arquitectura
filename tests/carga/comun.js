// Lo que comparten los escenarios de carga (ADR-013).
//
// Todos entran por NGINX, como un usuario real (BASE_URL). Los tokens se firman acá con
// JWT_SECRET, igual que Usuarios (15.5, 15.13), así las pruebas no dependen del login.

import crypto from 'k6/crypto';
import encoding from 'k6/encoding';

export const BASE = __ENV.BASE_URL || 'http://nginx';

function base64url(texto) {
  return encoding.b64encode(texto, 'rawurl');
}

// Arma un token válido por 1 hora. rol: "airline" u "operator".
export function token(rol, aerolinea) {
  const secreto = __ENV.JWT_SECRET;
  if (!secreto) {
    throw new Error('Falta JWT_SECRET: k6 lo necesita para firmar los tokens de prueba.');
  }
  const ahora = Math.floor(Date.now() / 1000);
  const datos = { sub: `carga-${aerolinea || rol}`, role: rol, iat: ahora, exp: ahora + 3600 };
  if (aerolinea) {
    datos.airline = aerolinea;
  }
  const cabecera = base64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const cuerpo = base64url(JSON.stringify(datos));
  const firma = crypto.hmac('sha256', secreto, `${cabecera}.${cuerpo}`, 'base64rawurl');
  return `${cabecera}.${cuerpo}.${firma}`;
}

export function conToken(tok, nombre) {
  return {
    headers: { Authorization: `Bearer ${tok}`, 'Content-Type': 'application/json' },
    tags: { name: nombre },
  };
}

// Un número de vuelo que no se repite entre corridas: dos letras de la aerolínea y 4 cifras.
export function numeroDeVuelo(aerolinea, n) {
  return `${aerolinea}${String(n % 10000).padStart(4, '0')}`;
}

// El comienzo de un bloque de 15 minutos (2.1), a partir de una fecha base y un índice.
export function bloque(base, indice) {
  return new Date(base.getTime() + indice * 15 * 60 * 1000).toISOString().replace('.000Z', 'Z');
}

// Una fecha base para los slots de una corrida: un día al azar entre 30 y 3000 días adelante,
// a las 00:00 UTC. Así dos corridas no piden los mismos bloques.
export function diaAlAzar() {
  const dias = 30 + Math.floor(Math.random() * 2970);
  const d = new Date(Date.now() + dias * 24 * 60 * 60 * 1000);
  d.setUTCHours(0, 0, 0, 0);
  return d;
}
