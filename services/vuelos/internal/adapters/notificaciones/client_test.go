package notificaciones

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
)

var mail = commands.Mail{IdempotencyKey: "evento-1", To: "ops@ar.test", Subject: "Vuelo AR1550 demorado", Body: "Texto"}

func TestSendArmaElPedido(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/notificaciones" {
			t.Errorf("pedido %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "clave" || r.Header.Get("Idempotency-Key") != "evento-1" {
			t.Errorf("encabezados: %v", r.Header)
		}
		var p map[string]string
		_ = json.NewDecoder(r.Body).Decode(&p)
		want := map[string]string{"canal": "email", "destinatario": "ops@ar.test", "asunto": "Vuelo AR1550 demorado", "cuerpo": "Texto"}
		for k, v := range want {
			if p[k] != v {
				t.Errorf("%s = %q, se esperaba %q", k, p[k], v)
			}
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"n-1","estado":"pendiente"}`))
	}))
	defer srv.Close()
	if err := NewClient(srv.URL+"/", "clave", time.Second).Send(context.Background(), mail); err != nil {
		t.Fatal(err)
	}
}

// Cualquier respuesta que no sea 202 vuelve como error, con el código del proveedor.
func TestSendErrorDelProveedor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"codigo":"no_disponible","mensaje":"Servicio temporalmente no disponible."}`))
	}))
	defer srv.Close()
	err := NewClient(srv.URL, "clave", time.Second).Send(context.Background(), mail)
	if err == nil || !strings.Contains(err.Error(), "503 no_disponible") {
		t.Fatalf("error: %v", err)
	}
}

// Si el proveedor no contesta a tiempo, el pedido se corta (27.5) y vuelve como error.
func TestSendTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	if err := NewClient(srv.URL, "clave", 100*time.Millisecond).Send(context.Background(), mail); err == nil {
		t.Fatal("se esperaba un error por timeout")
	}
}
