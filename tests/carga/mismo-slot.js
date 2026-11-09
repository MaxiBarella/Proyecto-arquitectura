// Escenario 3 del ADR-013: muchos pedidos al mismo tiempo por el mismo slot.
//
// Antes de empezar se publican VUELOS vuelos (por defecto 50), todos despegues, de AR y LA.
// Después cada uno pide, a la vez, la misma pista y el mismo bloque. Lo tiene que obtener uno
// solo (RN-10, 4.6): uno recibe 202 y el resto 409 con SLOT_TAKEN. Entre dos despegues no
// hay prioridad (RN-13), así que gana el primero que llega a la base.
//
// Si lo obtiene más de uno, el umbral "slot_obtenido count==1" falla y k6 termina con error.

import http from 'k6/http';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';
import { check } from 'k6';
import { BASE, bloque, conToken, diaAlAzar, numeroDeVuelo, token } from './comun.js';

const VUELOS = Number(__ENV.VUELOS || 50);
const obtenido = new Counter('slot_obtenido');
const rechazado = new Counter('slot_rechazado');

export const options = {
  scenarios: {
    mismo_slot: {
      // Cada usuario virtual hace un solo pedido, y arrancan todos juntos.
      executor: 'per-vu-iterations',
      vus: VUELOS,
      iterations: 1,
    },
  },
  thresholds: {
    slot_obtenido: ['count==1'],
    slot_rechazado: [`count==${VUELOS - 1}`],
  },
};

export function setup() {
  const tokens = { AR: token('airline', 'AR'), LA: token('airline', 'LA') };
  const inicio = bloque(diaAlAzar(), 40); // 10:00 UTC del día elegido
  const vuelos = [];
  for (let i = 0; i < VUELOS; i++) {
    const aerolinea = i % 2 === 0 ? 'AR' : 'LA';
    const r = http.post(
      `${BASE}/api/vuelos/flights`,
      JSON.stringify({
        flight_number: numeroDeVuelo(aerolinea, Date.now() + i),
        type: 'departure',
        scope: 'domestic',
        origin: 'Córdoba',
        destination: 'Buenos Aires',
        scheduled_at: inicio,
      }),
      conToken(tokens[aerolinea], 'publicar_vuelo'),
    );
    if (r.status !== 201) {
      throw new Error(`No pude publicar el vuelo ${i}: ${r.status} ${r.body}`);
    }
    vuelos.push({ id: r.json('id'), aerolinea });
  }
  return { tokens, inicio, vuelos };
}

export default function (datos) {
  const vuelo = datos.vuelos[exec.vu.idInTest - 1];
  const r = http.post(
    `${BASE}/api/vuelos/flights/${vuelo.id}/slot-request`,
    JSON.stringify({ runway_id: '05-23', block_start: datos.inicio }),
    conToken(datos.tokens[vuelo.aerolinea], 'pedido_mismo_slot'),
  );
  if (r.status === 202) {
    obtenido.add(1);
  } else if (r.status === 409 && r.json('code') === 'SLOT_TAKEN') {
    rechazado.add(1);
  }
  check(r, { '202 o 409 SLOT_TAKEN': (x) => x.status === 202 || x.status === 409 });
}
