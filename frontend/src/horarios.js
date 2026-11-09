// Horarios en hora de Argentina (UTC−3, sin horario de verano), que es la del aeropuerto
// (14.9, RN-33). La API manda y recibe en UTC; las pantallas muestran y piden en hora de
// Argentina, sin importar la zona de la computadora del que mira.

const ZONA = 'America/Argentina/Cordoba';

// "23/10 14:30"
export function horaCorta(iso) {
  return new Date(iso).toLocaleString('es-AR', {
    timeZone: ZONA,
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

// "14:30"
export function soloHora(iso) {
  return new Date(iso).toLocaleTimeString('es-AR', { timeZone: ZONA, hour: '2-digit', minute: '2-digit' });
}

// La fecha de hoy en Argentina, como AAAA-MM-DD (para los <input type="date">).
export function hoy() {
  return new Date().toLocaleDateString('en-CA', { timeZone: ZONA });
}

// Une una fecha (AAAA-MM-DD) y una hora (HH:MM) de Argentina en un instante UTC, con el
// formato que pide la API.
export function aUTC(fecha, hora) {
  return new Date(`${fecha}T${hora}:00-03:00`).toISOString().replace('.000Z', 'Z');
}

// Las horas de los bloques de 15 minutos (RN-03): 00:00, 00:15... 23:45.
export const BLOQUES = Array.from({ length: 96 }, (_, i) => {
  const h = String(Math.floor(i / 4)).padStart(2, '0');
  const m = String((i % 4) * 15).padStart(2, '0');
  return `${h}:${m}`;
});

// Nombres para mostrar.
export const TIPOS = { arrival: 'Arribo', departure: 'Despegue' };
export const AMBITOS = { domestic: 'Nacional', international: 'Internacional' };
