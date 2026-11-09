// La sesión del usuario: el token y lo que el frontend necesita saber de él.
//
// Se guarda en sessionStorage (22.7): sobrevive si se recarga la página y se borra al cerrar
// la pestaña. El token dura 1 hora (15.2); pasado ese tiempo la sesión se da por cerrada.
//
// Las pantallas la leen con el hook useSesion(), que vuelve a dibujar la pantalla cuando
// alguien inicia o cierra sesión.

import { useSyncExternalStore } from 'react';

const CLAVE = 'sesion';
const avisar = new Set();
let actual = leer();

function leer() {
  try {
    const guardada = JSON.parse(sessionStorage.getItem(CLAVE));
    if (guardada && guardada.vence > Date.now()) {
      return guardada;
    }
  } catch {
    // Sin sessionStorage o con algo roto guardado: se arranca sin sesión.
  }
  return null;
}

function guardar(sesion) {
  actual = sesion;
  try {
    if (sesion) {
      sessionStorage.setItem(CLAVE, JSON.stringify(sesion));
    } else {
      sessionStorage.removeItem(CLAVE);
    }
  } catch {
    // Si el navegador no deja guardar, la sesión dura hasta recargar.
  }
  avisar.forEach((funcion) => funcion());
}

// La sesión vigente, o null. Tiene { token, rol, aerolinea, vence }.
// rol es "airline" u "operator" (15.9); aerolinea es el código, solo para las aerolíneas.
export function sesionActual() {
  if (actual && actual.vence <= Date.now()) {
    guardar(null);
  }
  return actual;
}

// Guarda la respuesta de POST /api/usuarios/login (contrato de Usuarios, 15.11):
// { access_token, token_type, expires_in, role, airline }.
export function iniciarSesion(respuesta) {
  guardar({
    token: respuesta.access_token,
    rol: respuesta.role,
    aerolinea: respuesta.airline || null,
    vence: Date.now() + respuesta.expires_in * 1000,
  });
}

// Solo para desarrollo, mientras Usuarios no emite tokens (15.20): arma la sesión con un token
// de nginx/token-de-prueba.sh. Lee los datos del token sin verificarlo; el que verifica es NGINX.
export function iniciarSesionConToken(token) {
  const datos = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
  guardar({
    token,
    rol: datos.role,
    aerolinea: datos.airline || null,
    vence: datos.exp * 1000,
  });
}

export function cerrarSesion() {
  guardar(null);
}

function suscribir(funcion) {
  avisar.add(funcion);
  return () => avisar.delete(funcion);
}

// Hook para las pantallas: const sesion = useSesion();
export function useSesion() {
  return useSyncExternalStore(suscribir, sesionActual);
}
