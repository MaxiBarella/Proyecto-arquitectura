// El estado de un vuelo como etiqueta de color, con el nombre del SPEC.
// Los valores son los del contrato de Vuelos (FlightStatus).
const NOMBRES = {
  pending_slot: 'Pendiente de slot',
  scheduled: 'Programado',
  delayed: 'Demorado',
  completed: 'Finalizado',
  diverted: 'Desviado',
  cancelled: 'Cancelado',
};

export default function EstadoVuelo({ estado }) {
  return <span className={`estado estado-${estado}`}>{NOMBRES[estado] || estado}</span>;
}
