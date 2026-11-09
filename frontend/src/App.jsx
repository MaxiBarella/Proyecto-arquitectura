import { Route, Routes } from 'react-router';
import Plantilla from './componentes/Plantilla.jsx';
import SoloRol from './componentes/SoloRol.jsx';
import Tablero from './paginas/Tablero.jsx';
import Ingreso from './paginas/Ingreso.jsx';
import Aerolinea from './paginas/Aerolinea.jsx';
import Operador from './paginas/Operador.jsx';
import NoEncontrada from './paginas/NoEncontrada.jsx';

// Las pantallas del sistema (22.1). Cada una vive en su archivo de paginas/, así cada
// uno trabaja en el suyo sin pisarse. Para sumar una ruta, se agrega acá.
export default function App() {
  return (
    <Routes>
      <Route element={<Plantilla />}>
        <Route index element={<Tablero />} />
        <Route path="ingreso" element={<Ingreso />} />
        <Route element={<SoloRol rol="airline" />}>
          <Route path="aerolinea" element={<Aerolinea />} />
        </Route>
        <Route element={<SoloRol rol="operator" />}>
          <Route path="operador" element={<Operador />} />
        </Route>
        <Route path="*" element={<NoEncontrada />} />
      </Route>
    </Routes>
  );
}
