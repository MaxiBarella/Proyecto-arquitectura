package controllers

import "github.com/gin-gonic/gin"

// problem es un error en formato Problem Details (RFC 9457), con el campo code propio (6.14).
type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}

// Códigos de error del contrato (services/usuarios/openapi.yaml).
const (
	codeInvalidRequest     = "INVALID_REQUEST"
	codeInvalidCredentials = "INVALID_CREDENTIALS"
	codeInternalError      = "INTERNAL_ERROR"
)

var problemTypes = map[string]struct{ path, title string }{
	codeInvalidRequest:     {"/problems/invalid-request", "Pedido inválido"},
	codeInvalidCredentials: {"/problems/invalid-credentials", "Usuario o contraseña incorrectos"},
	codeInternalError:      {"/problems/internal-error", "Error interno"},
}

// writeProblem corta el pedido y responde el error.
func writeProblem(c *gin.Context, status int, code, detail string) {
	t := problemTypes[code]
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, problem{
		Type:     t.path,
		Title:    t.title,
		Status:   status,
		Detail:   detail,
		Instance: c.Request.URL.Path,
		Code:     code,
	})
}
