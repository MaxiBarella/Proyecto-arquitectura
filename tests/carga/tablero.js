// Escenario 1 del ADR-013: el tablero público, con la rampa de 10 a 1000 pedidos por segundo.
//
// Se corre dos veces, sin caché y con caché (ADR-007), y se compara. Un minuto de
// calentamiento a 10 por segundo y después un minuto en cada escalón: 10, 100, 500 y 1000
// (16.2). Es la medición del antes y el después de la caché, y muestra el reparto entre las
// dos instancias de Vuelos (ADR-012).
//
// La mezcla de consultas se configura con PORCENTAJE_PORTADA (por defecto 80, 10.8): ese
// porcentaje pide la portada del tablero y el resto, filtros al azar. Tiene que ser la misma
// en las dos corridas (ADR-007). Antes de la primera se corre sembrar-tablero.js (10.9).

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
const ORDENES = ['scheduled_at', 'airline', 'scope'];
// Comienzos de números de vuelo y de las ciudades de sembrar-tablero.js (RN-34).
const BUSQUEDAS = [
  'ar', 'la', 'ar1', 'ar2', 'ar3', 'ar4', 'ar5', 'ar6', 'ar7', 'ar8',
  'la1', 'la2', 'la3', 'la4', 'la5', 'la6', 'la7', 'la8',
  'cor', 'bue', 'men', 'sal', 'ros', 'bar', 'ush', 'tuc', 'neu',
  'san', 'lim', 'mon', 'mad', 'mia', 'pan', 'asu',
];

function alAzar(lista) {
  return lista[Math.floor(Math.random() * lista.length)];
}

// Un día de los próximos 7, en hora de Argentina (UTC−3), como lo filtra el tablero (RN-33).
function diaAlAzar() {
  const d = new Date(Date.now() - 3 * 60 * 60 * 1000 + Math.floor(Math.random() * 7) * 24 * 60 * 60 * 1000);
  return d.toISOString().slice(0, 10);
}

// El 20 % con filtros tiene que repetirse poco (10.10): con uno a tres filtros sobre tipo,
// estado, aerolínea, fecha, búsqueda, orden y página son miles de consultas distintas.
function filtrosAlAzar() {
  const filtros = [
    () => `type=${alAzar(TIPOS)}`,
    () => `status=${alAzar(ESTADOS)}`,
    () => `airline=${alAzar(AEROLINEAS)}`,
    () => `date=${diaAlAzar()}`,
    () => `q=${alAzar(BUSQUEDAS)}`,
    () => `sort=${alAzar(ORDENES)}`,
    () => `page=${1 + Math.floor(Math.random() * 5)}`,
  ];
  const cuantos = 1 + Math.floor(Math.random() * 3);
  return filtros
    .sort(() => Math.random() - 0.5)
    .slice(0, cuantos)
    .map((filtro) => filtro())
    .join('&');
}

export default function () {
  const portada = Math.random() * 100 < PORTADA;
  const url = portada ? `${BASE}/api/vuelos/board` : `${BASE}/api/vuelos/board?${filtrosAlAzar()}`;
  const respuesta = http.get(url, { tags: { name: 'tablero', portada: String(portada) } });
  check(respuesta, { 'tablero 200': (r) => r.status === 200 });
}
