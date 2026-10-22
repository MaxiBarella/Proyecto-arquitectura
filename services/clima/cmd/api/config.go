package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// config es la configuración de Clima, que sale de variables de entorno (9.15).
type config struct {
	port      string
	mongoURI  string
	apiKeys   map[string]string // clave → consumidor
	rabbitURL string
	// rabbitEnabled es false en la copia de la nube (9.3): no publica eventos ni
	// expone la ruta interna del forzado.
	rabbitEnabled bool
}

func loadConfig() (config, error) {
	cfg := config{
		port:      os.Getenv("PORT"),
		mongoURI:  os.Getenv("MONGO_URI"),
		rabbitURL: os.Getenv("RABBITMQ_URL"),
	}
	if cfg.port == "" {
		cfg.port = "8080"
	}
	if cfg.mongoURI == "" {
		return cfg, errors.New("falta la variable MONGO_URI")
	}

	enabled := os.Getenv("RABBITMQ_ENABLED")
	if enabled == "" {
		enabled = "true"
	}
	var err error
	if cfg.rabbitEnabled, err = strconv.ParseBool(enabled); err != nil {
		return cfg, fmt.Errorf("RABBITMQ_ENABLED tiene que ser true o false, no %q", enabled)
	}
	if cfg.rabbitEnabled && cfg.rabbitURL == "" {
		return cfg, errors.New("falta la variable RABBITMQ_URL (o RABBITMQ_ENABLED=false)")
	}

	if cfg.apiKeys, err = parseAPIKeys(os.Getenv("CLIMA_API_KEYS")); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// parseAPIKeys lee las claves con el formato grupo1:clave1,grupo2:clave2.
func parseAPIKeys(raw string) (map[string]string, error) {
	keys := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		consumer, key, ok := strings.Cut(entry, ":")
		consumer, key = strings.TrimSpace(consumer), strings.TrimSpace(key)
		if !ok || consumer == "" || key == "" {
			return nil, fmt.Errorf("CLIMA_API_KEYS: %q no tiene la forma consumidor:clave", entry)
		}
		if _, dup := keys[key]; dup {
			return nil, fmt.Errorf("CLIMA_API_KEYS: la clave de %q está repetida", consumer)
		}
		keys[key] = consumer
	}
	if len(keys) == 0 {
		return nil, errors.New("falta la variable CLIMA_API_KEYS, con la forma consumidor:clave")
	}
	return keys, nil
}
