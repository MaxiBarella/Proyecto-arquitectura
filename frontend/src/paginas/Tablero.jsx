import Aviso from '../componentes/Aviso.jsx';

// Tablero público de arribos y despegues. Lo arma Maxi (22.1) contra GET /api/vuelos/board,
// que pasa por el gateway sin token (15.7).
export default function Tablero() {
  return (
    <section>
      <h1>Tablero de vuelos</h1>
      <Aviso tipo="info">El tablero lo arma Maxi (22.1).</Aviso>
    </section>
  );
}
