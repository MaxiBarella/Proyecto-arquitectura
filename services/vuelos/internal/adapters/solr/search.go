package solr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/commands"
	"github.com/MaxiBarella/Proyecto-arquitectura/services/vuelos/internal/queries"
)

// sorts traduce el orden del tablero a Solr. Los empates van por horario y después
// por número de vuelo (RN-35), así la paginación no repite ni saltea vuelos.
var sorts = map[string]string{
	queries.SortByTime:    "board_at asc, flight_number asc",
	queries.SortByAirline: "airline asc, board_at asc, flight_number asc",
	queries.SortByScope:   "scope asc, board_at asc, flight_number asc",
}

// Search busca una página del tablero en el core (ADR-006).
func (c *Client) Search(ctx context.Context, s queries.BoardSearch) ([]queries.IndexedFlight, int64, error) {
	sort, ok := sorts[s.Sort]
	if !ok {
		return nil, 0, fmt.Errorf("orden desconocido: %q", s.Sort)
	}
	params := url.Values{}
	params.Set("wt", "json")
	params.Set("sort", sort)
	params.Set("rows", strconv.Itoa(queries.BoardPageSize))
	params.Set("start", strconv.Itoa((s.Page-1)*queries.BoardPageSize))
	if s.Query != "" {
		// Lo escrito va como un solo término del campo de búsqueda por el comienzo
		// (14.8). Va en un parámetro aparte, así no hay que escapar nada.
		params.Set("q", "{!field f=busqueda v=$texto}")
		params.Set("texto", s.Query)
	} else {
		params.Set("q", "*:*")
	}
	// Los filtros usan {!term}, que toma el valor tal cual, sin escapar.
	for field, value := range map[string]string{"type": s.Type, "status": s.Status, "airline": s.Airline} {
		if value != "" {
			params.Add("fq", fmt.Sprintf("{!term f=%s}%s", field, value))
		}
	}
	to := "*"
	if !s.To.IsZero() {
		to = s.To.UTC().Format(time.RFC3339)
	}
	params.Add("fq", fmt.Sprintf("board_at:[%s TO %s}", s.From.UTC().Format(time.RFC3339), to))

	// Por POST con el formulario, así una búsqueda larga no queda en la URL.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.coreURL+"/select", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.searches.Add(ctx, 1)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("buscar en Solr: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, 0, fmt.Errorf("buscar en Solr: respondió %d: %s", resp.StatusCode, detail)
	}
	var body struct {
		Response struct {
			NumFound int64      `json:"numFound"`
			Docs     []document `json:"docs"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, 0, fmt.Errorf("leer la respuesta de Solr: %w", err)
	}
	flights := make([]queries.IndexedFlight, len(body.Response.Docs))
	for i, d := range body.Response.Docs {
		flights[i] = fromDocument(d)
	}
	return flights, body.Response.NumFound, nil
}

func fromDocument(d document) queries.IndexedFlight {
	f := queries.IndexedFlight{
		FlightUpdatedData: commands.FlightUpdatedData{
			ID:           d.ID,
			FlightNumber: d.FlightNumber,
			Airline:      d.Airline,
			Type:         d.Type,
			Scope:        d.Scope,
			Origin:       d.Origin,
			Destination:  d.Destination,
			ScheduledAt:  d.ScheduledAt.UTC(),
			Status:       d.Status,
			DivertedTo:   d.DivertedTo,
			CreatedAt:    d.CreatedAt.UTC(),
			UpdatedAt:    d.UpdatedAt.UTC(),
		},
		BoardAt:     d.BoardAt.UTC(),
		DataVersion: d.DataVersion,
	}
	if d.SlotRunwayID != nil && d.SlotBlockStart != nil {
		f.Slot = &commands.FlightSlotData{RunwayID: *d.SlotRunwayID, BlockStart: d.SlotBlockStart.UTC()}
	}
	return f
}
