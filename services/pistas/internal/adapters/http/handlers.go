package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

// CasosDeUso es lo que el adaptador HTTP necesita del núcleo.
type CasosDeUso interface {
	Asignar(ctx context.Context, p domain.PedidoDeSlot) (domain.Resultado, error)
}

// NuevoRouter arma las rutas de services/pistas/openapi.yaml.
func NuevoRouter(casos CasosDeUso, log *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), correlacion(), registrarPedido(log))

	h := &handlers{casos: casos, log: log}
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pistas"})
	})
	// Ruta interna: la llama Vuelos, el gateway no la expone (4.15, 15.7).
	r.POST("/internal/slots", h.asignar)
	return r
}

type handlers struct {
	casos CasosDeUso
	log   *slog.Logger
}

type pedidoJSON struct {
	FlightID   string `json:"flight_id"`
	FlightType string `json:"flight_type"`
	RunwayID   string `json:"runway_id"`
	BlockStart string `json:"block_start"`
}

type slotJSON struct {
	RunwayID   string    `json:"runway_id"`
	BlockStart time.Time `json:"block_start"`
}

type asignacionJSON struct {
	FlightID          string    `json:"flight_id"`
	RunwayID          string    `json:"runway_id"`
	BlockStart        time.Time `json:"block_start"`
	DisplacedFlightID *string   `json:"displaced_flight_id"`
}

// problema es el formato Problem Details (RFC 9457) con el campo code propio (6.14).
type problema struct {
	Type             string     `json:"type"`
	Title            string     `json:"title"`
	Status           int        `json:"status"`
	Detail           string     `json:"detail,omitempty"`
	Code             string     `json:"code"`
	NearestFreeSlots []slotJSON `json:"nearest_free_slots,omitempty"`
}

func (h *handlers) asignar(c *gin.Context) {
	var p pedidoJSON
	if err := c.ShouldBindJSON(&p); err != nil {
		h.responderError(c, domain.ErrDatosInvalidos{Motivo: "el cuerpo no es un JSON válido"})
		return
	}
	inicio, err := time.Parse(time.RFC3339, p.BlockStart)
	if err != nil {
		h.responderError(c, domain.ErrDatosInvalidos{Motivo: "block_start tiene que ser una fecha RFC 3339"})
		return
	}
	res, err := h.casos.Asignar(c.Request.Context(), domain.PedidoDeSlot{
		VueloID:       p.FlightID,
		TipoOperacion: domain.TipoOperacion(p.FlightType),
		PistaID:       p.RunwayID,
		InicioBloque:  inicio,
	})
	if err != nil {
		h.responderError(c, err)
		return
	}
	// 200 tanto si se acaba de asignar como si el pedido se repitió (4.15).
	var desplazado *string
	if res.VueloDesplazadoID != "" {
		desplazado = &res.VueloDesplazadoID
	}
	c.JSON(http.StatusOK, asignacionJSON{
		FlightID:          res.Slot.VueloID,
		RunwayID:          res.Slot.PistaID,
		BlockStart:        res.Slot.InicioBloque.UTC(),
		DisplacedFlightID: desplazado,
	})
}

// responderError traduce los errores del núcleo a los códigos del contrato. Lo que no es un error
// de negocio se responde 503: es una falla pasajera (la base no responde, un choque que no se
// resolvió con los reintentos) y Vuelos la reintenta (4.5). Repetir el pedido es seguro (4.6).
func (h *handlers) responderError(c *gin.Context, err error) {
	var invalidos domain.ErrDatosInvalidos
	var ocupado domain.ErrSlotOcupado
	switch {
	case errors.As(err, &invalidos):
		responder(c, problema{Type: "/problems/invalid-request", Title: "Pedido inválido",
			Status: http.StatusBadRequest, Detail: invalidos.Motivo, Code: "INVALID_REQUEST"})
	case errors.Is(err, domain.ErrPistaInexistente):
		responder(c, problema{Type: "/problems/runway-not-found", Title: "Pista no encontrada",
			Status: http.StatusNotFound, Code: "RUNWAY_NOT_FOUND"})
	case errors.Is(err, domain.ErrPistaCerrada):
		responder(c, problema{Type: "/problems/runway-closed", Title: "Pista cerrada",
			Status: http.StatusConflict, Code: "RUNWAY_CLOSED"})
	case errors.Is(err, domain.ErrVueloConSlot):
		responder(c, problema{Type: "/problems/flight-already-has-slot", Title: "El vuelo ya tiene un slot",
			Status: http.StatusConflict, Code: "FLIGHT_ALREADY_HAS_SLOT"})
	case errors.As(err, &ocupado):
		libres := make([]slotJSON, len(ocupado.Libres))
		for i, l := range ocupado.Libres {
			libres[i] = slotJSON{RunwayID: l.PistaID, BlockStart: l.InicioBloque.UTC()}
		}
		responder(c, problema{Type: "/problems/slot-taken", Title: "Slot ocupado",
			Status: http.StatusConflict, Code: "SLOT_TAKEN", NearestFreeSlots: libres})
	default:
		h.log.ErrorContext(c.Request.Context(), "falla al atender el pedido", "error", err,
			"correlation_id", aplicacion.CorrelacionDe(c.Request.Context()))
		responder(c, problema{Type: "/problems/unavailable", Title: "Pistas no está disponible",
			Status: http.StatusServiceUnavailable, Detail: "Se puede reintentar.", Code: "UNAVAILABLE"})
	}
}

func responder(c *gin.Context, p problema) {
	c.Header("Content-Type", "application/problem+json")
	c.JSON(p.Status, p)
}
