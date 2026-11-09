import Aviso from '../componentes/Aviso.jsx';

// Pantalla del operador: pistas, cerrar y reabrir, forzar el clima y el aviso de Clima sin
// datos. La arma Carola (22.1). Solo entra el rol operator (SoloRol en App.jsx).
export default function Operador() {
  return (
    <section>
      <h1>Operación</h1>
      <Aviso tipo="info">La pantalla del operador la arma Carola (22.1).</Aviso>
    </section>
  );
}
