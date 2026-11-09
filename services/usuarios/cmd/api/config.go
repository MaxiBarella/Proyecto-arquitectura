package main

import (
	"fmt"
	"os"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/services"
)

// config es la configuración de Usuarios, que sale de variables de entorno.
type config struct {
	port     string
	mysqlDSN string
	// jwtSecret firma los tokens; NGINX verifica con la misma (15.5, 15.17).
	jwtSecret string
	accounts  []services.Account
}

func loadConfig() (config, error) {
	cfg := config{
		port:      os.Getenv("PORT"),
		mysqlDSN:  os.Getenv("MYSQL_DSN"),
		jwtSecret: os.Getenv("JWT_SECRET"),
	}
	if cfg.port == "" {
		cfg.port = "8080"
	}
	if cfg.mysqlDSN == "" {
		return cfg, fmt.Errorf("falta la variable MYSQL_DSN")
	}
	if cfg.jwtSecret == "" {
		return cfg, fmt.Errorf("falta la variable JWT_SECRET")
	}

	// Las cuentas de prueba (15.15): un operador y las aerolíneas AR y LA.
	accounts := []struct {
		env string
		services.Account
	}{
		{"USUARIOS_OPERADOR_PASSWORD", services.Account{Username: "operador", Role: models.RoleOperator}},
		{"USUARIOS_AEROLINEAS_PASSWORD", services.Account{Username: "aerolineas", Role: models.RoleAirline, Airline: "AR"}},
		{"USUARIOS_LATAM_PASSWORD", services.Account{Username: "latam", Role: models.RoleAirline, Airline: "LA"}},
	}
	for _, a := range accounts {
		a.Password = os.Getenv(a.env)
		if a.Password == "" {
			return cfg, fmt.Errorf("falta la variable %s", a.env)
		}
		cfg.accounts = append(cfg.accounts, a.Account)
	}
	return cfg, nil
}
