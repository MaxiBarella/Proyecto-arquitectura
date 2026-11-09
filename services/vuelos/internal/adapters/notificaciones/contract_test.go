package notificaciones

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Test de contrato con el proveedor (ADR-009): levanta un mock con Prism a partir de
// su contrato, testdata/notificaciones.yaml, que valida cada pedido contra el
// contrato y responde 400 si no lo cumple. Si el proveedor publica una versión nueva,
// se reemplaza el archivo y este test dice si lo que mandamos sigue valiendo.
// Necesita Docker; con -short se saltea.
func TestContratoConElProveedor(t *testing.T) {
	if testing.Short() {
		t.Skip("test de contrato: se saltea con -short")
	}
	ctx := context.Background()
	spec, err := filepath.Abs("testdata/notificaciones.yaml")
	if err != nil {
		t.Fatal(err)
	}
	mock, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "stoplight/prism:5",
			Cmd:   []string{"mock", "-h", "0.0.0.0", "-p", "4010", "-m", "false", "/contrato/notificaciones.yaml"},
			Files: []testcontainers.ContainerFile{
				{HostFilePath: spec, ContainerFilePath: "/contrato/notificaciones.yaml", FileMode: 0o644},
			},
			ExposedPorts: []string{"4010/tcp"},
			WaitingFor:   wait.ForLog("Prism is listening").WithStartupTimeout(time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("no se pudo levantar el mock (¿Docker no está disponible?): %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(mock) }()
	endpoint, err := mock.PortEndpoint(ctx, "4010/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(endpoint, "prueba", 10*time.Second)

	t.Run("el pedido que mandamos cumple el contrato", func(t *testing.T) {
		m := mail
		m.IdempotencyKey = "6d6c5c1e-0b7a-4f3e-9a2b-1c2d3e4f5a6b"
		if err := client.Send(ctx, m); err != nil {
			t.Fatalf("el mock rechazó el pedido: %v", err)
		}
	})

	t.Run("un pedido que no cumple el contrato se rechaza", func(t *testing.T) {
		// El asunto pasa los 150 caracteres que permite el contrato: el mock tiene que
		// responder 400. Comprueba que el test de arriba de verdad valida algo.
		m := mail
		m.Subject = strings.Repeat("x", 151)
		err := client.Send(ctx, m)
		if err == nil || !strings.Contains(err.Error(), fmt.Sprint(400)) {
			t.Fatalf("se esperaba un 400 del mock, vino: %v", err)
		}
	})
}
