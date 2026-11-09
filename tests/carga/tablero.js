// Escenario 1 del ADR-013: el tablero público, con la rampa de 10 a 1000 pedidos por segundo.
//
// Se corre dos veces, sin caché y con caché (ADR-007), y se compara. Un minuto de
// calentamiento a 10 por segundo y después un minuto en cada escalón: 10, 100, 500 y 1000
// (16.2). Es la medición del antes y el después de la caché, y muestra el reparto entre las
// dos instancias de Vuelos (ADR-012).
//
// La mezcla de consultas se configura con PORCENTAJE_PORTADA (por defecto 80): ese porcentaje
// pide la portada del tablero y el resto, filtros al azar. Tiene que ser la misma en las dos
// corridas (ADR-007).

import http from 'k6/http';
import { check } from 'k6';
import { BASE } from './comun.js';

const PORTADA = Number(__ENV.PORCENTAJE_PORTADA || 80);

// Cada escalón: 5 segundos para subir y 1 minuto sostenido.
function escalon(objetivo) {
  return [
    { target: objetivo, duration: '5s' },
    { target: objetivo, duration: '1m' },
  ];
}

export const options = {
  scenarios: {
    tablero: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 2000,
      stages: [
        { target: 10, duration: '1m' }, // calentamiento
        ...escalon(10),
        ...escalon(100),
        ...escalon(500),
        ...escalon(1000),
      ],
    },
  },
  thresholds: {
    // El objetivo de servicio es del pedido de slot (11.3); para el tablero se usa como
    // referencia: el escalón donde se pasa es el límite (ADR-013).
    'http_req_duration{name:tablero}': ['p(95)<500'],
    'http_req_failed{name:tablero}': ['rate<0.01'],
  },
};

const TIPOS = ['arrival', 'departure'];
const ESTADOS = ['scheduled', 'delayed', 'pending_slot'];
const AEROLINEAS = ['AR', 'LA'];
const BUSQUEDAS = ['ar', 'la', 'cor', 'bue', 'men'];

function filtrosAlAzar() {
  const filtros = [
    `type=${TIPOS[Math.floor(Math.random() * TIPOS.length)]}`,
    `status=${ESTADOS[Math.floor(Math.random() * ESTADOS.length)]}`,
    `airline=${AEROLINEAS[Math.floor(Math.random() * AEROLINEAS.length)]}`,
    `q=${BUSQUEDAS[Math.floor(Math.random() * BUSQUEDAS.length)]}`,
    `page=${1 + Math.floor(Math.random() * 3)}`,
  ];
  // Uno o dos filtros, para que haya muchas combinaciones distintas.
  const cuantos = 1 + Math.floor(Math.random() * 2);
  return filtros.sort(() => Math.random() - 0.5).slice(0, cuantos).join('&');
}

export default function () {
  const portada = Math.random() * 100 < PORTADA;
  const url = portada ? `${BASE}/api/vuelos/board` : `${BASE}/api/vuelos/board?${filtrosAlAzar()}`;
  const respuesta = http.get(url, { tags: { name: 'tablero', portada: String(portada) } });
  check(respuesta, { 'tablero 200': (r) => r.status === 200 });
}
