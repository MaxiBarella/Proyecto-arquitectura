package rabbitmq

import (
	"context"
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
)

// confirmador registra qué hizo el consumidor con el mensaje.
type confirmador struct {
	ack, nack, reencolar bool
}

func (c *confirmador) Ack(uint64, bool) error { c.ack = true; return nil }
func (c *confirmador) Nack(_ uint64, _ bool, reencolar bool) error {
	c.nack, c.reencolar = true, reencolar
	return nil
}
func (c *confirmador) Reject(uint64, bool) error { return nil }

func entrega(c *confirmador, cuerpo string) amqp.Delivery {
	return amqp.Delivery{Acknowledger: c, RoutingKey: "vuelos.vuelo.cancelado", Body: []byte(cuerpo)}
}

const mensajeValido = `{"id":"e1","type":"vuelos.vuelo.cancelado","version":1,"data":{"flight_id":"f1"}}`

func consumidorDePrueba(p Procesador) *Consumidor {
	return &Consumidor{procesador: p, esperas: []time.Duration{0, 0, 0}}
}

func TestConfirmaLoAplicado(t *testing.T) {
	c := &confirmador{}
	var recibido aplicacion.Mensaje
	consumidorDePrueba(func(_ context.Context, m aplicacion.Mensaje) error { recibido = m; return nil }).
		procesar(context.Background(), entrega(c, mensajeValido))
	if !c.ack || recibido.ID != "e1" || string(recibido.Data) != `{"flight_id":"f1"}` {
		t.Fatalf("se esperaba ack con el mensaje leído: %+v, %+v", c, recibido)
	}
}

// 4.5, 4.24 y 4.20: una falla pasajera se reintenta 3 veces; si sigue, nack sin reencolar, y
// RabbitMQ lo pasa a la cola de fallidos.
func TestDespuesDeTresReintentosVaAFallidos(t *testing.T) {
	c := &confirmador{}
	intentos := 0
	consumidorDePrueba(func(context.Context, aplicacion.Mensaje) error { intentos++; return errors.New("MySQL no responde") }).
		procesar(context.Background(), entrega(c, mensajeValido))
	if intentos != 4 {
		t.Fatalf("se esperaban 4 intentos (1 + 3 reintentos), hubo %d", intentos)
	}
	if !c.nack || c.reencolar || c.ack {
		t.Fatalf("se esperaba nack sin reencolar: %+v", c)
	}
}

func TestUnaFallaPasajeraQueSeResuelve(t *testing.T) {
	c := &confirmador{}
	intentos := 0
	consumidorDePrueba(func(context.Context, aplicacion.Mensaje) error {
		intentos++
		if intentos < 3 {
			return errors.New("MySQL no responde")
		}
		return nil
	}).procesar(context.Background(), entrega(c, mensajeValido))
	if !c.ack || intentos != 3 {
		t.Fatalf("se esperaba ack al tercer intento: %+v, %d intentos", c, intentos)
	}
}

// 4.22: lo inaplicable se confirma, sin reintentos ni fallidos.
func TestInaplicableSeConfirma(t *testing.T) {
	for nombre, cuerpo := range map[string]string{"cuerpo roto": `{no es json`, "sin id": `{"type":"x"}`} {
		c := &confirmador{}
		intentos := 0
		consumidorDePrueba(func(context.Context, aplicacion.Mensaje) error { intentos++; return nil }).
			procesar(context.Background(), entrega(c, cuerpo))
		if !c.ack || c.nack || intentos != 0 {
			t.Errorf("%s: se esperaba ack sin procesar: %+v, %d intentos", nombre, c, intentos)
		}
	}
}

// Si Pistas se está apagando, el mensaje vuelve a la cola para no perderse.
func TestAlApagarseVuelveALaCola(t *testing.T) {
	c := &confirmador{}
	ctx, cancelar := context.WithCancel(context.Background())
	consumidorDePrueba(func(context.Context, aplicacion.Mensaje) error { cancelar(); return errors.New("cortado") }).
		procesar(ctx, entrega(c, mensajeValido))
	if !c.nack || !c.reencolar {
		t.Fatalf("se esperaba nack con reencolar: %+v", c)
	}
}
