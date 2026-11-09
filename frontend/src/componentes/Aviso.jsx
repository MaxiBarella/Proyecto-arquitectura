// Muestra un error de la API (ErrorApi) o cualquier mensaje, con el estilo común.
//   <Aviso error={error} />          rojo, con el mensaje del error
//   <Aviso tipo="ok">Listo</Aviso>  verde
//   <Aviso tipo="info">...</Aviso>  azul
export default function Aviso({ error, tipo = 'error', children }) {
  if (!error && !children) {
    return null;
  }
  return (
    <p className={`aviso aviso-${tipo}`} role={tipo === 'error' ? 'alert' : 'status'}>
      {children || error.message}
      {error?.codigo && <code className="aviso-codigo">{error.codigo}</code>}
    </p>
  );
}
