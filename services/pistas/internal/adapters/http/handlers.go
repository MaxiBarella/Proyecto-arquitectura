package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/aplicacion"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

// CasosDeUso es lo que el adaptador HTTP necesita del núcleo.
type CasosDeUso interface {
	Asignar(ctx context.Context, p domain.PedidoDeSlot) (domain.Resultado, error)
	ListarSlots(ctx context.Context, fecha time.Time, pistaID int64) ([]aplicacion.BloqueDelDia, error)
	ListarPistas(ctx context.Context) ([]domain.Pista, error)
}

// NuevoRouter arma las rutas de docs/contracts/pistas.yaml que se programan en la Entrega 2.
func NuevoRouter(casos CasosDeUso, log *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), correlacion(), registrarPedido(log))

	h := &handlers{casos: casos, log: log}
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pistas"})
	})
	r.POST("/asignaciones", h.asignar)
	r.GET("/slots", h.listarSlots)
	r.GET("/pistas", h.listarPistas)
	return r
}

type handlers struct {
	casos CasosDeUso
	log   *slog.Logger
}

type pedidoJSON struct {
	VueloID       string `json:"vuelo_id"`
	TipoOperacion string `json:"tipo_operacion"`
	PistaID       int64  `json:"pista_id"`
	InicioBloque  string `json:"inicio_bloque"`
}

type slotJSON struct {
	PistaID      int64  `json:"pista_id"`
	InicioBloque string `json:"inicio_bloque"`
}

type asignacionJSON struct {
	SlotID            int64  `json:"slot_id"`
	VueloID           string `json:"vuelo_id"`
	TipoOperacion     string `json:"tipo_operacion"`
	PistaID           int64  `json:"pista_id"`
	InicioBloque      string `json:"inicio_bloque"`
	VueloDesplazadoID string `json:"vuelo_desplazado_id,omitempty"`
}

type errorJSON struct {
	Error       string     `json:"error"`
	Mensaje     string     `json:"mensaje"`
	SlotsLibres []slotJSON `json:"slots_libres,omitempty"`
}

func (h *handlers) asignar(c *gin.Context) {
	var p pedidoJSON
	if err := c.ShouldBindJSON(&p); err != nil {
		h.responderError(c, domain.ErrDatosInvalidos{Motivo: "el cuerpo no es un JSON válido"})
		return
	}
	inicio, err := time.Parse(time.RFC3339, p.InicioBloque)
	if err != nil {
		h.responderError(c, domain.ErrDatosInvalidos{Motivo: "inicio_bloque tiene que estar en formato RFC 3339"})
		return
	}
	res, err := h.casos.Asignar(c.Request.Context(), domain.PedidoDeSlot{
		VueloID:       p.VueloID,
		TipoOperacion: domain.TipoOperacion(p.TipoOperacion),
		PistaID:       p.PistaID,
		InicioBloque:  inicio,
	})
	if err != nil {
		h.responderError(c, err)
		return
	}
	estado := http.StatusCreated
	if res.YaAsignado {
		estado = http.StatusOK
	}
	c.JSON(estado, asignacionJSON{
		SlotID:            res.Slot.ID,
		VueloID:           res.Slot.VueloID,
		TipoOperacion:     string(res.Slot.TipoOperacion),
		PistaID:           res.Slot.PistaID,
		InicioBloque:      horario(res.Slot.InicioBloque),
		VueloDesplazadoID: res.VueloDesplazadoID,
	})
}

func (h *handlers) listarSlots(c *gin.Context) {
	fecha, err := time.ParseInLocation(time.DateOnly, c.Query("fecha"), domain.ZonaCordoba)
	if err != nil {
		h.responderError(c, domain.ErrDatosInvalidos{Motivo: "fecha tiene que ser AAAA-MM-DD"})
		return
	}
	var pistaID int64
	if v := c.Query("pista_id"); v != "" {
		pistaID, err = strconv.ParseInt(v, 10, 64)
		if err != nil || pistaID <= 0 {
			h.responderError(c, domain.ErrDatosInvalidos{Motivo: "pista_id tiene que ser un número"})
			return
		}
	}
	bloques, err := h.casos.ListarSlots(c.Request.Context(), fecha, pistaID)
	if err != nil {
		h.responderError(c, err)
		return
	}
	type bloqueJSON struct {
		slotJSON
		Libre bool `json:"libre"`
	}
	res := make([]bloqueJSON, len(bloques))
	for i, b := range bloques {
		res[i] = bloqueJSON{slotJSON: slotJSON{PistaID: b.PistaID, InicioBloque: horario(b.InicioBloque)}, Libre: b.Libre}
	}
	c.JSON(http.StatusOK, gin.H{"fecha": fecha.Format(time.DateOnly), "bloques": res})
}

func (h *handlers) listarPistas(c *gin.Context) {
	pistas, err := h.casos.ListarPistas(c.Request.Context())
	if err != nil {
		h.responderError(c, err)
		return
	}
	type pistaJSON struct {
		ID      int64  `json:"id"`
		Nombre  string `json:"nombre"`
		Abierta bool   `json:"abierta"`
	}
	res := make([]pistaJSON, len(pistas))
	for i, p := range pistas {
		res[i] = pistaJSON{ID: p.ID, Nombre: p.Nombre, Abierta: p.Abierta}
	}
	c.JSON(http.StatusOK, res)
}

// responderError traduce los errores del núcleo a los códigos del contrato. Lo que no es un error
// de negocio se responde 503: es una falla transitoria (la base no responde, un choque que no se
// resolvió con los reintentos) y Vuelos puede reintentar (4.5). Repetir el pedido es seguro (4.6).
func (h *handlers) responderError(c *gin.Context, err error) {
	var invalidos domain.ErrDatosInvalidos
	var ocupado domain.ErrSlotOcupado
	switch {
	case errors.As(err, &invalidos):
		c.JSON(http.StatusBadRequest, errorJSON{Error: "datos_invalidos", Mensaje: mayuscula(invalidos.Motivo)})
	case errors.Is(err, domain.ErrPistaInexistente):
		c.JSON(http.StatusNotFound, errorJSON{Error: "pista_inexistente", Mensaje: "La pista no existe."})
	case errors.Is(err, domain.ErrPistaCerrada):
		c.JSON(http.StatusConflict, errorJSON{Error: "pista_cerrada", Mensaje: "La pista está cerrada."})
	case errors.Is(err, domain.ErrVueloConSlot):
		c.JSON(http.StatusConflict, errorJSON{Error: "vuelo_con_slot", Mensaje: "El vuelo ya tiene otro slot asignado."})
	case errors.As(err, &ocupado):
		libres := make([]slotJSON, len(ocupado.Libres))
		for i, l := range ocupado.Libres {
			libres[i] = slotJSON{PistaID: l.PistaID, InicioBloque: horario(l.InicioBloque)}
		}
		c.JSON(http.StatusConflict, errorJSON{Error: "slot_ocupado", Mensaje: "El slot ya está ocupado.", SlotsLibres: libres})
	default:
		h.log.ErrorContext(c.Request.Context(), "falla al atender el pedido", "error", err,
			"correlacion_id", aplicacion.CorrelacionDe(c.Request.Context()))
		c.JSON(http.StatusServiceUnavailable, errorJSON{Error: "no_disponible", Mensaje: "Pistas no puede atender el pedido ahora. Se puede reintentar."})
	}
}

func horario(t time.Time) string {
	return t.In(domain.ZonaCordoba).Format(time.RFC3339)
}

func mayuscula(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:] + "."
}
