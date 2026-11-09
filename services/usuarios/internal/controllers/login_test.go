package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/models"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/usuarios/internal/services"
)

type fakeAuth struct {
	session models.Session
	err     error
}

func (f fakeAuth) Login(context.Context, string, string) (models.Session, error) {
	return f.session, f.err
}

func post(t *testing.T, auth Authenticator, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", NewLoginController(auth).Login)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body)))
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("la respuesta no es JSON: %s", w.Body.String())
	}
	return w, out
}

func TestLogin200Aerolinea(t *testing.T) {
	auth := fakeAuth{session: models.Session{AccessToken: "tok", ExpiresIn: 3600, Role: "airline", Airline: "AR"}}
	w, out := post(t, auth, `{"username":"aerolineas","password":"x"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	want := map[string]any{"access_token": "tok", "token_type": "Bearer", "expires_in": float64(3600), "role": "airline", "airline": "AR"}
	for k, v := range want {
		if out[k] != v {
			t.Errorf("%s = %v, se esperaba %v", k, out[k], v)
		}
	}
}

func TestLogin200OperadorSinAerolinea(t *testing.T) {
	auth := fakeAuth{session: models.Session{AccessToken: "tok", ExpiresIn: 3600, Role: "operator"}}
	_, out := post(t, auth, `{"username":"operador","password":"x"}`)
	if _, ok := out["airline"]; ok {
		t.Fatal("la respuesta del operador no tiene que traer airline (15.11)")
	}
}

func TestLogin400(t *testing.T) {
	for name, body := range map[string]string{
		"no es JSON":     `usuario=x`,
		"sin usuario":    `{"password":"x"}`,
		"sin contraseña": `{"username":"x"}`,
	} {
		w, out := post(t, fakeAuth{}, body)
		if w.Code != http.StatusBadRequest || out["code"] != codeInvalidRequest {
			t.Errorf("%s: status %d, code %v", name, w.Code, out["code"])
		}
	}
}

func TestLogin401(t *testing.T) {
	w, out := post(t, fakeAuth{err: services.ErrInvalidCredentials}, `{"username":"x","password":"y"}`)
	if w.Code != http.StatusUnauthorized || out["code"] != codeInvalidCredentials {
		t.Fatalf("status %d, code %v", w.Code, out["code"])
	}
	if w.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("Content-Type %q", w.Header().Get("Content-Type"))
	}
}

func TestLogin500SinDetalle(t *testing.T) {
	w, out := post(t, fakeAuth{err: errors.New("MySQL no responde")}, `{"username":"x","password":"y"}`)
	if w.Code != http.StatusInternalServerError || out["code"] != codeInternalError {
		t.Fatalf("status %d, code %v", w.Code, out["code"])
	}
	if _, ok := out["detail"]; ok {
		t.Fatal("el 500 no tiene que contar qué falló")
	}
}
