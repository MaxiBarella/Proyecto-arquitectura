package domain

import (
	"errors"
	"fmt"
)

// Errores de negocio. El adaptador HTTP los traduce a los códigos de docs/contracts/pistas.yaml.
var (
	ErrPistaInexistente = errors.New("la pista no existe")
	ErrPistaCerrada     = errors.New("la pista está cerrada")
	ErrVueloConSlot     = errors.New("el vuelo ya tiene otro slot asignado")
	// ErrPistaYaExiste: el alta de una pista con un nombre que ya está (23.5).
	ErrPistaYaExiste = errors.New("ya existe una pista con ese nombre")
	// ErrCerradaPorClima: el operador quiere reabrir una pista mientras Clima tiene datos y la
	// pista la cerró el clima (RN-42, 23.4) o el clima no permite operar (23.6).
	ErrCerradaPorClima = errors.New("el clima no permite reabrir la pista a mano")
	// ErrAlternativoInexistente: el código no está en la lista de alternativos (RN-25, 23.5).
	ErrAlternativoInexistente = errors.New("el aeropuerto no está en la lista de alternativos")
)

// ErrDatosInvalidos indica un pedido mal formado.
type ErrDatosInvalidos struct {
	Motivo string
}

func (e ErrDatosInvalidos) Error() string {
	return "datos inválidos: " + e.Motivo
}

// ErrSlotOcupado indica que el slot pedido tiene otro vuelo y no corresponde la prioridad del arribo.
// Trae los slots libres más cercanos (RN-09).
type ErrSlotOcupado struct {
	Libres []SlotLibre
}

func (e ErrSlotOcupado) Error() string {
	return fmt.Sprintf("el slot está ocupado; hay %d libres cercanos", len(e.Libres))
}
