#!/bin/sh
# Corre el escenario de la caída (caida.js): apaga vuelos-2 al minuto y la prende al segundo.
# Uso, desde la raíz del repo y con el sistema levantado:
#   tests/carga/caida.sh
#   RUTA=/api/vuelos/flights tests/carga/caida.sh   (otra ruta de Vuelos, con token)
set -eu

# En Git Bash de Windows, que no convierta /carga y /resultados en rutas de Windows.
export MSYS_NO_PATHCONV=1

docker compose --profile carga run --rm \
    -e TASA="${TASA:-100}" -e RUTA="${RUTA:-/api/vuelos/board}" \
    k6 run --summary-export /resultados/caida.json /carga/caida.js &
k6=$!

sleep 60
echo ">>> $(date +%T) apago vuelos-2"
docker compose stop vuelos-2
sleep 60
echo ">>> $(date +%T) prendo vuelos-2"
docker compose start vuelos-2

wait $k6
