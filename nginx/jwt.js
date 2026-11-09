// Verifica el token de cada pedido sin preguntarle a Usuarios (15.2, 15.4).
//
// Lo usa gateway.conf a través de cuatro variables:
//   $jwt_sub, $jwt_role y $jwt_airline: los datos del token, o vacías si no hay un token válido.
//   $jwt_motivo: por qué se rechazó el token, para el log. Vacía si es válido.
//
// El token es un JWT firmado con HS256 y la clave JWT_SECRET (15.5, 15.17), con estos datos
// (15.13): sub, role ("airline" u "operator", 15.9), airline (solo las aerolíneas), iat y exp.

const crypto = require('crypto');

// HS256 pide una clave de al menos 256 bits (RFC 7518, sección 3.2). Decisión 15.21.
const LARGO_MINIMO_CLAVE = 32;
const ROLES = ['airline', 'operator'];
const BEARER = /^Bearer ([A-Za-z0-9_-]+)\.([A-Za-z0-9_-]+)\.([A-Za-z0-9_-]+)$/;

function leerJson(parte) {
    return JSON.parse(Buffer.from(parte, 'base64url').toString());
}

// Compara las firmas sin cortar en la primera diferencia, así el tiempo de respuesta
// no da pistas sobre cuántos caracteres coinciden.
function igualesSinFiltrarTiempo(a, b) {
    if (a.length !== b.length) {
        return false;
    }
    let diferencia = 0;
    for (let i = 0; i < a.length; i++) {
        diferencia |= a.charCodeAt(i) ^ b.charCodeAt(i);
    }
    return diferencia === 0;
}

// Devuelve { datos } si el token es válido, o { motivo } si no.
function verificar(r) {
    const clave = process.env.JWT_SECRET || '';
    if (Buffer.from(clave).length < LARGO_MINIMO_CLAVE) {
        // No debería pasar: el contenedor no arranca con una clave corta (verifica-jwt-secret.sh).
        return { motivo: 'clave del servidor muy corta' };
    }

    const encabezado = r.headersIn.Authorization;
    if (!encabezado) {
        return { motivo: 'sin token' };
    }
    const partes = BEARER.exec(encabezado);
    if (!partes) {
        return { motivo: 'formato invalido' };
    }

    let cabecera;
    let datos;
    try {
        cabecera = leerJson(partes[1]);
        datos = leerJson(partes[2]);
    } catch (e) {
        return { motivo: 'formato invalido' };
    }

    // Solo HS256. Así nadie puede mandar un token con "alg": "none" y sin firma.
    if (!cabecera || cabecera.alg !== 'HS256') {
        return { motivo: 'algoritmo no permitido' };
    }

    const firma = crypto.createHmac('sha256', clave)
        .update(partes[1] + '.' + partes[2])
        .digest('base64url');
    if (!igualesSinFiltrarTiempo(firma, partes[3])) {
        return { motivo: 'firma invalida' };
    }

    if (!datos || typeof datos.exp !== 'number' || datos.exp <= Math.floor(Date.now() / 1000)) {
        return { motivo: 'vencido' };
    }
    if (typeof datos.sub !== 'string' || datos.sub === '') {
        return { motivo: 'sin sub' };
    }
    if (ROLES.indexOf(datos.role) === -1) {
        return { motivo: 'rol desconocido' };
    }
    if (datos.role === 'airline' && (typeof datos.airline !== 'string' || datos.airline === '')) {
        return { motivo: 'aerolinea sin codigo' };
    }

    return {
        datos: {
            sub: datos.sub,
            role: datos.role,
            // Un operador no tiene aerolínea, aunque el token traiga una.
            airline: datos.role === 'airline' ? datos.airline : '',
        },
    };
}

function sub(r) {
    const resultado = verificar(r);
    return resultado.datos ? resultado.datos.sub : '';
}

function role(r) {
    const resultado = verificar(r);
    return resultado.datos ? resultado.datos.role : '';
}

function airline(r) {
    const resultado = verificar(r);
    return resultado.datos ? resultado.datos.airline : '';
}

function motivo(r) {
    const resultado = verificar(r);
    return resultado.motivo || '';
}

export default { sub, role, airline, motivo };
