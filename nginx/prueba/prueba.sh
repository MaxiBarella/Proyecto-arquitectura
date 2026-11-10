#!/usr/bin/env bash
# Prueba el gateway: rutas, token, identidad hacia los servicios y límite de tráfico.
#
# Levanta NGINX delante de un servicio falso que responde qué le llegó (compose.yml),
# corre los casos y lo baja. Necesita Docker, curl y openssl.
#
# Uso, desde la raíz del repo:   nginx/prueba/prueba.sh
set -uo pipefail

cd "$(dirname "$0")"
GATEWAY=http://localhost:18080
CLAVE=clave-de-la-prueba-del-gateway-0123456789
fallas=0
casos=0

docker compose up -d --build --quiet-pull >/dev/null 2>&1 || {
    echo "No pude levantar la prueba. Probá a mano: docker compose -f nginx/prueba/compose.yml up --build"
    exit 1
}
trap 'docker compose down >/dev/null 2>&1' EXIT

for _ in $(seq 30); do
    curl -s -o /dev/null "$GATEWAY/" && break
    sleep 0.5
done

base64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }

# firmar <cabecera JSON> <datos JSON> [clave]
firmar() {
    local cabecera datos firma
    cabecera=$(printf '%s' "$1" | base64url)
    datos=$(printf '%s' "$2" | base64url)
    firma=$(printf '%s' "$cabecera.$datos" | openssl dgst -sha256 -hmac "${3:-$CLAVE}" -binary | base64url)
    echo "$cabecera.$datos.$firma"
}

HS256='{"alg":"HS256","typ":"JWT"}'
AHORA=$(date +%s)
EXP=$((AHORA + 3600))
AEROLINEA=$(firmar "$HS256" "{\"sub\":\"7\",\"role\":\"airline\",\"airline\":\"AR\",\"iat\":$AHORA,\"exp\":$EXP}")
OPERADOR=$(firmar "$HS256" "{\"sub\":\"1\",\"role\":\"operator\",\"airline\":\"AR\",\"iat\":$AHORA,\"exp\":$EXP}")

# caso <nombre> <estado esperado> <texto esperado en la respuesta> <argumentos de curl...>
caso() {
    local nombre=$1 estado=$2 texto=$3
    shift 3
    casos=$((casos + 1))
    local respuesta codigo cuerpo
    respuesta=$(curl -s -w '\n%{http_code}' "$@")
    codigo=$(tail -n 1 <<<"$respuesta")
    cuerpo=$(sed '$d' <<<"$respuesta")
    if [[ "$codigo" == "$estado" && "$cuerpo" == *"$texto"* ]]; then
        echo "ok    $nombre"
    else
        fallas=$((fallas + 1))
        echo "FALLA $nombre"
        echo "      esperaba $estado con: $texto"
        echo "      llegó    $codigo con: $cuerpo"
    fi
}

echo "== Rutas sin token (15.7)"
caso "el tablero pasa sin token" 200 '"servicio":"vuelos","uri":"/board?page=2"' \
    "$GATEWAY/api/vuelos/board?page=2"
caso "el tablero pisa la identidad que manda el cliente" 200 '"user_id":"","role":"","airline":""' \
    -H 'X-User-Id: 99' -H 'X-User-Role: operator' -H 'X-Airline: XX' "$GATEWAY/api/vuelos/board"
caso "el inicio de sesión pasa sin token" 200 '"servicio":"usuarios","uri":"/login"' \
    -X POST -d '{}' "$GATEWAY/api/usuarios/login"

echo "== Rutas con token"
caso "sin token responde 401" 401 '"code":"INVALID_TOKEN"' "$GATEWAY/api/vuelos/flights"
caso "el 401 trae WWW-Authenticate" 401 'WWW-Authenticate: Bearer' -i "$GATEWAY/api/vuelos/flights"
caso "el 401 es Problem Details" 401 'application/problem+json' -i "$GATEWAY/api/vuelos/flights"
caso "aerolínea: llega a Vuelos sin el prefijo" 200 '"servicio":"vuelos","uri":"/flights?status=scheduled"' \
    -H "Authorization: Bearer $AEROLINEA" "$GATEWAY/api/vuelos/flights?status=scheduled"
caso "aerolínea: Vuelos recibe su identidad (15.6)" 200 '"user_id":"7","role":"airline","airline":"AR"' \
    -H "Authorization: Bearer $AEROLINEA" "$GATEWAY/api/vuelos/flights"
caso "aerolínea: se pisa la identidad que manda el cliente" 200 '"user_id":"7","role":"airline","airline":"AR"' \
    -H "Authorization: Bearer $AEROLINEA" -H 'X-User-Id: 99' -H 'X-User-Role: operator' -H 'X-Airline: LA' \
    "$GATEWAY/api/vuelos/flights"
caso "operador: no lleva X-Airline aunque el token o el cliente la manden" 200 '"user_id":"1","role":"operator","airline":""' \
    -H "Authorization: Bearer $OPERADOR" -H 'X-Airline: LA' "$GATEWAY/api/clima/internal/airports/SACO/forced-conditions"
caso "operador: Clima recibe la ruta sin el prefijo" 200 '"servicio":"clima","uri":"/internal/airports/SACO/forced-conditions"' \
    -H "Authorization: Bearer $OPERADOR" "$GATEWAY/api/clima/internal/airports/SACO/forced-conditions"
caso "las rutas de Pistas van a Pistas" 200 '"servicio":"pistas","uri":"/runways"' \
    -H "Authorization: Bearer $OPERADOR" "$GATEWAY/api/pistas/runways"

echo "== Tokens rechazados"
caso "vencido" 401 INVALID_TOKEN -H "Authorization: Bearer $(firmar "$HS256" \
    "{\"sub\":\"7\",\"role\":\"airline\",\"airline\":\"AR\",\"iat\":$((AHORA - 7200)),\"exp\":$((AHORA - 3600))}")" \
    "$GATEWAY/api/vuelos/flights"
caso "firmado con otra clave" 401 INVALID_TOKEN -H "Authorization: Bearer $(firmar "$HS256" \
    "{\"sub\":\"7\",\"role\":\"airline\",\"airline\":\"AR\",\"exp\":$EXP}" otra-clave-de-32-bytes-o-mas-1234567)" \
    "$GATEWAY/api/vuelos/flights"
caso "datos cambiados después de firmar" 401 INVALID_TOKEN -H "Authorization: Bearer $(
    cab=${AEROLINEA%%.*}; fir=${AEROLINEA##*.}
    dat=$(printf '%s' "{\"sub\":\"7\",\"role\":\"operator\",\"exp\":$EXP}" | base64url)
    echo "$cab.$dat.$fir")" "$GATEWAY/api/vuelos/flights"
caso "algoritmo none" 401 INVALID_TOKEN -H "Authorization: Bearer $(
    printf '%s' '{"alg":"none"}' | base64url).$(printf '%s' "{\"sub\":\"7\",\"role\":\"operator\",\"exp\":$EXP}" | base64url)." \
    "$GATEWAY/api/vuelos/flights"
caso "algoritmo none con firma de relleno" 401 INVALID_TOKEN -H "Authorization: Bearer $(firmar '{"alg":"none"}' \
    "{\"sub\":\"7\",\"role\":\"operator\",\"exp\":$EXP}")" "$GATEWAY/api/vuelos/flights"
caso "rol desconocido" 401 INVALID_TOKEN -H "Authorization: Bearer $(firmar "$HS256" \
    "{\"sub\":\"7\",\"role\":\"admin\",\"exp\":$EXP}")" "$GATEWAY/api/vuelos/flights"
caso "aerolínea sin código" 401 INVALID_TOKEN -H "Authorization: Bearer $(firmar "$HS256" \
    "{\"sub\":\"7\",\"role\":\"airline\",\"exp\":$EXP}")" "$GATEWAY/api/vuelos/flights"
caso "sin sub" 401 INVALID_TOKEN -H "Authorization: Bearer $(firmar "$HS256" \
    "{\"role\":\"operator\",\"exp\":$EXP}")" "$GATEWAY/api/vuelos/flights"
caso "sin la palabra Bearer" 401 INVALID_TOKEN -H "Authorization: $AEROLINEA" "$GATEWAY/api/vuelos/flights"
caso "basura" 401 INVALID_TOKEN -H "Authorization: Bearer no.es.untoken" "$GATEWAY/api/vuelos/flights"

echo "== Ruta interna de Pistas (15.19)"
caso "bloqueada aun con token" 404 '"code":"NOT_FOUND"' -X POST \
    -H "Authorization: Bearer $OPERADOR" "$GATEWAY/api/pistas/internal/slots"
caso "bloqueada con doble barra" 404 NOT_FOUND --path-as-is -X POST \
    -H "Authorization: Bearer $OPERADOR" "$GATEWAY/api/pistas//internal/slots"
caso "bloqueada con ../" 404 NOT_FOUND --path-as-is -X POST \
    -H "Authorization: Bearer $OPERADOR" "$GATEWAY/api/pistas/x/../internal/slots"
caso "bloqueada con letras codificadas" 404 NOT_FOUND -X POST \
    -H "Authorization: Bearer $OPERADOR" "$GATEWAY/api/pistas/%69nternal/slots"

echo "== Otras rutas"
caso "un servicio que no existe da 404" 404 NOT_FOUND -H "Authorization: Bearer $OPERADOR" "$GATEWAY/api/otro/x"
caso "/api sin servicio da 404" 404 NOT_FOUND "$GATEWAY/api/"

echo "== Frontend (15.7)"
caso "la raíz sirve el frontend" 200 '<div id="root">' "$GATEWAY/"
caso "una pantalla de React Router también" 200 '<div id="root">' "$GATEWAY/operador"
caso "index.html no se guarda en el navegador" 200 'Cache-Control: no-cache' -i "$GATEWAY/aerolinea"
casos=$((casos + 1))
archivo=$(curl -s "$GATEWAY/" | grep -o '/assets/[^"]*\.js' | head -n 1)
encabezados=$(curl -s -o /dev/null -D - "$GATEWAY$archivo")
if [[ -n "$archivo" ]] && grep -qi 'cache-control: public, max-age=31536000, immutable' <<<"$encabezados"; then
    echo "ok    los archivos de Vite se guardan en el navegador ($archivo)"
else
    fallas=$((fallas + 1))
    echo "FALLA no encontré el JavaScript del frontend o no tiene Cache-Control: $archivo"
fi

echo "== Límite de tráfico (15.8)"
casos=$((casos + 1))
# Se espera a que se recupere el cupo que usaron los casos anteriores (20 por segundo).
sleep 3
# 80 pedidos de golpe desde la misma IP, en una sola conexión.
# El cuerpo del último, que tiene que ser un 429, queda en un archivo.
ultimo=$(mktemp)
urls=()
for _ in $(seq 79); do urls+=(-o /dev/null "$GATEWAY/api/vuelos/board"); done
urls+=(-o "$ultimo" "$GATEWAY/api/vuelos/board")
codigos=$(curl -s -w '%{http_code}\n' "${urls[@]}")
ok=$(grep -c '^200$' <<<"$codigos")
limitados=$(grep -c '^429$' <<<"$codigos")
cuerpo=$(cat "$ultimo"); rm -f "$ultimo"
if [[ $ok -ge 40 && $limitados -ge 1 && $((ok + limitados)) -eq 80 && "$cuerpo" == *RATE_LIMIT_EXCEEDED* ]]; then
    echo "ok    de 80 pedidos de golpe pasan $ok y $limitados reciben 429 con RATE_LIMIT_EXCEEDED"
else
    fallas=$((fallas + 1))
    echo "FALLA límite: pasaron $ok, 429: $limitados, último cuerpo: $cuerpo"
fi

echo "== Log del gateway"
casos=$((casos + 1))
ultimo_401=$(docker compose logs gateway 2>/dev/null | grep '"status":401' | tail -n 1)
if [[ "$ultimo_401" == *'"token":"formato invalido"'* ]]; then
    echo "ok    el log en JSON dice por qué se rechazó el token: $(grep -o '"token":"[^"]*"' <<<"$ultimo_401")"
else
    fallas=$((fallas + 1))
    echo "FALLA no encontré el motivo del rechazo en el log: $ultimo_401"
fi

echo "== Clave de firma corta (15.21)"
casos=$((casos + 1))
imagen=$(docker compose images gateway --format json 2>/dev/null | grep -o '"Repository":"[^"]*"' | head -n 1 | cut -d'"' -f4)
salida=$(docker run --rm -e JWT_SECRET=corta "${imagen:-gateway-prueba-gateway}" 2>&1)
if [[ $? -ne 0 && "$salida" == *"hacen falta al menos 32"* ]]; then
    echo "ok    con una clave de 6 bytes NGINX no arranca"
else
    fallas=$((fallas + 1))
    echo "FALLA NGINX arrancó con una clave corta: $salida"
fi

# Va al final porque apaga el servicio falso.
echo "== Servicio caído (15.23)"
docker compose stop servicio-falso >/dev/null 2>&1
# Mientras NGINX todavía tiene la IP del contenedor, no conecta y responde 504 al segundo
# (12.5). Cuando el nombre deja de existir, responde 502 al instante. Los dos en Problem Details.
caido_primero=""
caido_502=""
for _ in $(seq 20); do
    respuesta=$(curl -s -i -H "Authorization: Bearer $AEROLINEA" "$GATEWAY/api/vuelos/flights")
    estado=$(head -n 1 <<<"$respuesta" | cut -d' ' -f2)
    [[ -z "$caido_primero" ]] && caido_primero="$estado $respuesta"
    if [[ "$estado" == 502 ]]; then
        caido_502="$respuesta"
        break
    fi
    sleep 0.5
done
casos=$((casos + 1))
if [[ "$caido_primero" =~ ^50[24] && "$caido_primero" == *'application/problem+json'* \
    && "$caido_primero" == *'"code":"SERVICE_UNAVAILABLE"'* ]]; then
    echo "ok    recién caído: ${caido_primero%% *} en Problem Details con SERVICE_UNAVAILABLE"
else
    fallas=$((fallas + 1))
    echo "FALLA recién caído, llegó: $caido_primero"
fi
casos=$((casos + 1))
if [[ "$caido_502" == *'application/problem+json'* && "$caido_502" == *'"status":502'* \
    && "$caido_502" == *'"code":"SERVICE_UNAVAILABLE"'* ]]; then
    echo "ok    después: 502 en Problem Details con SERVICE_UNAVAILABLE"
else
    fallas=$((fallas + 1))
    echo "FALLA no llegó un 502 en Problem Details en 10 segundos"
fi
caso "con los servicios caídos, el frontend se sigue viendo" 200 '<div id="root">' "$GATEWAY/"

echo
echo "$((casos - fallas)) de $casos casos bien."
[[ $fallas -eq 0 ]]
