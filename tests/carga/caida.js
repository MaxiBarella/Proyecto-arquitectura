// Escenario 4 del ADR-013: una instancia de Vuelos se cae en medio de la carga.
//
// Manda TASA pedidos por segundo (por defecto 100) durante 3 minutos. Lo corre caida.sh, que
// apaga vuelos-2 al minuto y la vuelve a prender al segundo minuto. En Grafana se ve cómo
// vuelos-1 absorbe todo y cómo vuelve el reparto (ADR-012). Sirve para el POSTMORTEM.md.
//
// Por defecto consulta el tablero. Con RUTA se puede usar otra ruta de Vuelos que pida token,
// por ejemplo RUTA=/api/vuelos/flights, con un token de aerolínea.

import http from 'k6/http';
import { check } from 'k6';
import { BASE, conToken, token } from './comun.js';

const TASA = Number(__ENV.TASA || 100);
const RUTA = __ENV.RUTA || '/api/vuelos/board';

export const options = {
  scenarios: {
    caida: {
      executor: 'constant-arrival-rate',
      rate: TASA,
      timeUnit: '1s',
      duration: '3m',
      preAllocatedVUs: 50,
      maxVUs: 1000,
    },
  },
  thresholds: {
    // Con una instancia caída, los pedidos tienen que seguir respondiendo (ADR-012).
    'http_req_failed{name:caida}': ['rate<0.01'],
  },
};

export function setup() {
  return { token: RUTA === '/api/vuelos/board' ? null : token('airline', 'AR') };
}

export default function (datos) {
  const opciones = datos.token ? conToken(datos.token, 'caida') : { tags: { name: 'caida' } };
  const r = http.get(`${BASE}${RUTA}`, opciones);
  check(r, { 'responde 200': (x) => x.status === 200 });
}
