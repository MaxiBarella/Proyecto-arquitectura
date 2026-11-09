-- Tabla de Usuarios (ADR-003, 5.3). MySQL corre este archivo una sola vez, cuando
-- crea la base. Si cambia, hay que borrar el volumen: docker compose down -v.
-- Las cuentas no se cargan acá: las carga el servicio al arrancar, con las
-- contraseñas del .env (15.15).

CREATE TABLE users (
  id            CHAR(36)    NOT NULL,
  username      VARCHAR(64) NOT NULL,
  -- Hash de bcrypt (15.14): siempre ocupa 60 caracteres.
  password_hash CHAR(60)    NOT NULL,
  role          VARCHAR(16) NOT NULL,
  -- Código de la aerolínea, solo si el rol es airline.
  airline       VARCHAR(8)  NULL,
  created_at    DATETIME(6) NOT NULL,
  updated_at    DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_users_username (username),
  CONSTRAINT chk_users_role CHECK (role IN ('airline', 'operator')),
  CONSTRAINT chk_users_airline CHECK ((role = 'airline') = (airline IS NOT NULL))
);
