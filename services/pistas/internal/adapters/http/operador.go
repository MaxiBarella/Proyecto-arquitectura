package httpadapter

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/pistas/internal/domain"
)

// Las rutas del operador: /runways y /alternate-airports (23.2, 23.3). Pasan por el gateway, que
// pone la identidad en X-User-Id y X-User-Role (15.6).

const rolOperador = "operator"

// soloOperador deja pasar solo al operador (CA-19). Sin identidad responde 401 y con otro rol, 403,
// como Vuelos.
func soloOperador() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-User-Id") == "" || c.GetHeader("X-User-Role") == "" {
			responder(c, problema{Type: "/problems/missing-identity", Title: "Falta la identidad",
				Status: http.StatusUnauthorized, Detail: "El pedido no incluye X-User-Id o X-User-Role.",
				Code: "MISSING_IDENTITY"})
			c.Abort()
			return
		}
		if c.GetHeader("X-User-Role") != rolOperador {
			responder(c, problema{Type: "/problems/forbidden-role", Title: "Operación no permitida para el rol",
				Status: http.StatusForbidden, Detail: "Solo el operador puede administrar las pistas.",
				Code: "FORBIDDEN_ROLE"})
			c.Abort()
			return
		}
		c.Next()
	}
}

type pistaJSON struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	ClosedReason *string `json:"closed_reason"`
}

type climaJSON struct {
	DataAvailable bool  `json:"data_available"`
	Operable      *bool `json:"operable"`
}

type alternativoJSON struct {
	ICAO      string `json:"icao"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

func aPistaJSON(p domain.Pista) pistaJSON {
	if p.Abierta {
		return pistaJSON{ID: p.ID, Status: "open"}
	}
	motivo := string(p.CerradaPor)
	return pistaJSON{ID: p.ID, Status: "closed", ClosedReason: &motivo}
}

func aAlternativoJSON(a domain.Alternativo) alternativoJSON {
	return alternativoJSON{ICAO: a.ICAO, Name: a.Nombre, Available: a.Disponible}
}

func (h *handlers) listarPistas(c *gin.Context) {
	pistas, clima, err := h.casos.ListarPistas(c.Request.Context())
	if err != nil {
		h.responderError(c, err)
		return
	}
	lista := make([]pistaJSON, len(pistas))
	for i, p := range pistas {
		lista[i] = aPistaJSON(p)
	}
	c.JSON(http.StatusOK, gin.H{
		"runways": lista,
		"weather": climaJSON{DataAvailable: clima.ConDatos, Operable: clima.Operable},
	})
}

func (h *handlers) altaDePista(c *gin.Context) {
	var cuerpo struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		h.responderError(c, domain.ErrDatosInvalidos{Motivo: "el cuerpo tiene que ser un JSON con id"})
		return
	}
	p, err := h.casos.AltaDePista(c.Request.Context(), cuerpo.ID)
	if err != nil {
		h.responderError(c, err)
		return
	}
	c.JSON(http.StatusCreated, aPistaJSON(p))
}

func (h *handlers) cerrarPista(c *gin.Context) {
	p, err := h.casos.CerrarPista(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, aPistaJSON(p))
}

func (h *handlers) reabrirPista(c *gin.Context) {
	p, err := h.casos.ReabrirPista(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.responderError(c, err)
		return
	}
	c.JSON(http.StatusOK, aPistaJSON(p))
}

func (h *handlers) listarAlternativos(c *gin.Context) {
	alts, err := h.casos.ListarAlternativos(c.Request.Context())
	if err != nil {
		h.responderError(c, err)
		return
	}
	lista := make([]alternativoJSON, len(alts))
	for i, a := range alts {
		lista[i] = aAlternativoJSON(a)
	}
	c.JSON(http.StatusOK, gin.H{"alternate_airports": lista})
}

func (h *handlers) cambiarDisponibilidad(disponible bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		a, err := h.casos.CambiarDisponibilidad(c.Request.Context(), c.Param("icao"), disponible)
		if err != nil {
			h.responderError(c, err)
			return
		}
		c.JSON(http.StatusOK, aAlternativoJSON(a))
	}
}
