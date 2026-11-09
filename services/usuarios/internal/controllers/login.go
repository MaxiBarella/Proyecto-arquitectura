package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/services"
)

// Authenticator es lo que el controller necesita del servicio.
type Authenticator interface {
	Login(ctx context.Context, username, password string) (models.Session, error)
}

type LoginController struct {
	auth Authenticator
}

func NewLoginController(auth Authenticator) *LoginController {
	return &LoginController{auth: auth}
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sessionResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Role        string `json:"role"`
	Airline     string `json:"airline,omitempty"`
}

// Login responde POST /login (15.10).
func (ctl *LoginController) Login(c *gin.Context) {
	var in credentials
	if err := json.NewDecoder(c.Request.Body).Decode(&in); err != nil {
		writeProblem(c, http.StatusBadRequest, codeInvalidRequest, "El cuerpo tiene que ser un JSON con username y password.")
		return
	}
	switch {
	case in.Username == "":
		writeProblem(c, http.StatusBadRequest, codeInvalidRequest, "Falta el usuario.")
		return
	case in.Password == "":
		writeProblem(c, http.StatusBadRequest, codeInvalidRequest, "Falta la contraseña.")
		return
	}

	ctx := c.Request.Context()
	session, err := ctl.auth.Login(ctx, in.Username, in.Password)
	if errors.Is(err, services.ErrInvalidCredentials) {
		// El log no lleva la contraseña; el usuario sí, para poder seguir un intento.
		slog.InfoContext(ctx, "inicio de sesión rechazado", "username", in.Username)
		writeProblem(c, http.StatusUnauthorized, codeInvalidCredentials, "")
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "no se pudo iniciar sesión", "error", err)
		writeProblem(c, http.StatusInternalServerError, codeInternalError, "")
		return
	}

	slog.InfoContext(ctx, "inicio de sesión", "username", in.Username, "role", session.Role)
	c.JSON(http.StatusOK, sessionResponse{
		AccessToken: session.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   session.ExpiresIn,
		Role:        session.Role,
		Airline:     session.Airline,
	})
}
