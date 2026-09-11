package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type MaintenanceWindow struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	MonitorIDs  []string  `json:"monitor_ids"` // empty = all monitors
	Active      bool      `json:"active"`
}

func (s *Server) ListMaintenance(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	// Windows that ended over 30 days ago drop off the list: still in the
	// table for the record, but nobody is managing them.
	rows, err := s.DB.Query(r.Context(), `
		SELECT w.id, w.title, w.description, w.starts_at, w.ends_at,
		       now() BETWEEN w.starts_at AND w.ends_at,
		       COALESCE(array_agg(mm.monitor_id::text)
		                FILTER (WHERE mm.monitor_id IS NOT NULL), '{}')
		FROM maintenance_windows w
		LEFT JOIN maintenance_monitors mm ON mm.window_id = w.id
		WHERE w.tenant_id = $1 AND w.ends_at > now() - interval '30 days'
		GROUP BY w.id
		ORDER BY w.starts_at DESC`, c.TenantID)
	if err != nil {
		slog.Error("list maintenance", "err", err)
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()

	out := []MaintenanceWindow{}
	for rows.Next() {
		var m MaintenanceWindow
		if rows.Scan(&m.ID, &m.Title, &m.Description, &m.StartsAt, &m.EndsAt,
			&m.Active, &m.MonitorIDs) == nil {
			out = append(out, m)
		}
	}
	writeJSON(w, 200, out)
}

type maintenanceReq struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	MonitorIDs  []string  `json:"monitor_ids"`
}

func (s *Server) CreateMaintenance(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)

	var in maintenanceReq
	if err := decode(r, &in); err != nil || in.Title == "" {
		writeErr(w, 400, "title, starts_at and ends_at are required")
		return
	}
	if !in.EndsAt.After(in.StartsAt) {
		writeErr(w, 400, "the window must end after it starts")
		return
	}
	// A window already in the past cannot suppress anything and is almost
	// certainly a mistake in the date field.
	if in.EndsAt.Before(time.Now().Add(-24 * time.Hour)) {
		writeErr(w, 400, "that window has already passed")
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var id string
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO maintenance_windows (tenant_id, title, description, starts_at, ends_at, created_by)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6) RETURNING id`,
		c.TenantID, in.Title, in.Description, in.StartsAt, in.EndsAt, c.UserID).Scan(&id); err != nil {
		slog.Error("create maintenance window", "err", err)
		writeErr(w, 400, "could not create window")
		return
	}

	// The tenant filter lives in the INSERT itself, so a caller cannot attach
	// a window to another tenant's monitor by guessing an ID.
	for _, mid := range in.MonitorIDs {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO maintenance_monitors (window_id, monitor_id)
			SELECT $1, id FROM monitors WHERE id=$2 AND tenant_id=$3`,
			id, mid, c.TenantID); err != nil {
			writeErr(w, 400, "invalid monitor in selection")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "db")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

func (s *Server) DeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	tag, err := s.DB.Exec(r.Context(),
		`DELETE FROM maintenance_windows WHERE id=$1 AND tenant_id=$2`,
		chi.URLParam(r, "id"), c.TenantID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, 404, "not found")
		return
	}
	w.WriteHeader(204)
}
