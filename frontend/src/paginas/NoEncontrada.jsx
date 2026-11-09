import { Link } from 'react-router';

export default function NoEncontrada() {
  return (
    <section>
      <h1>No existe esta pantalla</h1>
      <p><Link to="/">Volver al tablero</Link></p>
    </section>
  );
}
