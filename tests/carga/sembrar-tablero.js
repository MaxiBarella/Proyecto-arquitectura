// Carga los vuelos sobre los que se mide la caché del tablero (10.9, ADR-007).
//
// Se corre una sola vez, antes de las dos corridas de tablero.js, así las dos miden sobre los
// mismos datos. Publica 700 vuelos en los próximos 7 días, unos 100 por día, de las dos
// aerolíneas, arribos y despegues, nacionales e internacionales, y a 3 de cada 4 les pide
// slot. Cada vuelo usa su propio bloque de 15 minutos, alternando las pistas, así ningún
// pedido choca con otro.
//
// Al terminar espera 70 segundos: el índice confirma los cambios en 60 como máximo (14.2).

import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { BASE, bloque, conToken, numeroDeVuelo, token } from './comun.js';

const VUELOS = Number(__ENV.VUELOS || 700);
const PISTAS = ['05-23', '18-36'];
const AEROLINEAS = ['AR', 'LA'];
const NACIONALES = ['Buenos Aires', 'Mendoza', 'Salta', 'Rosario', 'Bariloche', 'Ushuaia', 'Tucumán', 'Neuquén'];
const INTERNACIONALES = ['Santiago', 'Lima', 'San Pablo', 'Montevideo', 'Madrid', 'Miami', 'Panamá', 'Asunción'];

export const options = {
  // La espera del final (teardown) es de 70 segundos; k6 corta a los 60 si no se le dice.
  teardownTimeout: '90s',
  scenarios: {
    sembrar: {
      executor: 'shared-iterations',
      vus: 10,
      iterations: VUELOS,
      maxDuration: '5m',
    },
  },
  thresholds: {
    'checks{check:vuelo publicado 201}': ['rate==1'],
    'checks{check:slot asignado 202}': ['rate==1'],
  },
};

export function setup() {
  // El primer bloque, a la próxima hora en punto más una: ningún vuelo queda en el pasado
  // mientras se publica (RN-41).
  const inicio = new Date();
  inicio.setUTCMinutes(0, 0, 0);
  inicio.setTime(inicio.getTime() + 2 * 60 * 60 * 1000);
  // Números de vuelo distintos en cada corrida del script.
  const desde = Math.floor(Math.random() * 9000);
  return {
    inicio: inicio.toISOString(),
    desde,
    tokens: { AR: token('airline', 'AR'), LA: token('airline', 'LA') },
  };
}

export default function (datos) {
  const i = exec.scenario.iterationInTest;
  const aerolinea = AEROLINEAS[i % AEROLINEAS.length];
  // Un vuelo por bloque de 15 minutos: 700 bloques son unos 7 días.
  const horario = bloque(new Date(datos.inicio), i);
  const arribo = i % 4 >= 2;
  const internacional = i % 3 === 0;
  const ciudades = internacional ? INTERNACIONALES : NACIONALES;
  const otra = ciudades[Math.floor(i / 4) % ciudades.length];
  const opciones = (nombre) => conToken(datos.tokens[aerolinea], nombre);

  const publicado = http.post(
    `${BASE}/api/vuelos/flights`,
    JSON.stringify({
      flight_number: numeroDeVuelo(aerolinea, datos.desde + i),
      type: arribo ? 'arrival' : 'departure',
      scope: internacional ? 'international' : 'domestic',
      origin: arribo ? otra : 'Córdoba',
      destination: arribo ? 'Córdoba' : otra,
      scheduled_at: horario,
    }),
    opciones('publicar_vuelo'),
  );
  if (!check(publicado, { 'vuelo publicado 201': (r) => r.status === 201 })) {
    return;
  }

  // 1 de cada 4 queda pendiente de slot, para que el filtro por estado encuentre de los dos.
  // Se toman de a grupos de 4 seguidos, así hay pendientes de los dos tipos y aerolíneas.
  if (Math.floor(i / 4) % 4 === 3) {
    return;
  }
  const pedido = http.post(
    `${BASE}/api/vuelos/flights/${publicado.json('id')}/slot-request`,
    JSON.stringify({ runway_id: PISTAS[i % PISTAS.length], block_start: horario }),
    opciones('pedido_slot'),
  );
  check(pedido, { 'slot asignado 202': (r) => r.status === 202 });
}

export function teardown() {
  sleep(70);
}
