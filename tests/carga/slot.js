// Escenario 2 del ADR-013: aerolíneas que publican vuelos y piden slots distintos.
//
// Cada iteración publica un vuelo y pide un slot libre para él. La rampa es de 5, 20 y 50
// pedidos de slot por segundo, un minuto cada uno (16.3). Sirve para ver hasta dónde se
// cumple el objetivo de servicio: el 95 % de los pedidos de slot en menos de 500 ms (11.3).
//
// Los slots no se repiten: cada iteración usa otro bloque de 15 minutos, alternando las dos
// pistas, a partir de un día al azar. Así ningún pedido choca con otro.

import http from 'k6/http';
import exec from 'k6/execution';
import { check } from 'k6';
import { BASE, bloque, conToken, diaAlAzar, numeroDeVuelo, token } from './comun.js';

const PISTAS = ['05-23', '18-36'];
const AEROLINEAS = ['AR', 'LA'];

function escalon(objetivo) {
  return [
    { target: objetivo, duration: '5s' },
    { target: objetivo, duration: '1m' },
  ];
}

export const options = {
  scenarios: {
    slot: {
      executor: 'ramping-arrival-rate',
      startRate: 5,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 500,
      stages: [...escalon(5), ...escalon(20), ...escalon(50)],
    },
  },
  thresholds: {
    // Si falla la publicación, el pedido de slot no se hace y la medición no sirve.
    'checks{check:vuelo publicado 201}': ['rate==1'],
    // Objetivo de servicio (11.3).
    'http_req_duration{name:pedido_slot}': ['p(95)<500'],
    'http_req_failed{name:pedido_slot}': ['rate<0.01'],
  },
};

export function setup() {
  return {
    dia: diaAlAzar().toISOString(),
    tokens: { AR: token('airline', 'AR'), LA: token('airline', 'LA') },
  };
}

export default function (datos) {
  const i = exec.scenario.iterationInTest;
  const aerolinea = AEROLINEAS[i % AEROLINEAS.length];
  const inicio = bloque(new Date(datos.dia), Math.floor(i / PISTAS.length));
  const opciones = (nombre) => conToken(datos.tokens[aerolinea], nombre);

  // Mitad despegues y mitad arribos. Un arribo llega a Córdoba y un despegue sale de ahí.
  const arribo = i % 4 >= 2;
  const publicado = http.post(
    `${BASE}/api/vuelos/flights`,
    JSON.stringify({
      flight_number: numeroDeVuelo(aerolinea, i),
      type: arribo ? 'arrival' : 'departure',
      scope: 'domestic',
      origin: arribo ? 'Buenos Aires' : 'Córdoba',
      destination: arribo ? 'Córdoba' : 'Buenos Aires',
      scheduled_at: inicio,
    }),
    opciones('publicar_vuelo'),
  );
  if (!check(publicado, { 'vuelo publicado 201': (r) => r.status === 201 })) {
    return;
  }

  const pedido = http.post(
    `${BASE}/api/vuelos/flights/${publicado.json('id')}/slot-request`,
    JSON.stringify({ runway_id: PISTAS[i % PISTAS.length], block_start: inicio }),
    opciones('pedido_slot'),
  );
  check(pedido, { 'slot asignado 202': (r) => r.status === 202 });
}
