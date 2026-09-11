package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Which channels a monitor alerts on. An empty list means "all of the tenant's
// channels", which is the sensible default: a user who has not thought about
// routing wants to hear about everything, not nothing.
//
// The distinction matters for a monitor whose channels have all been deleted —
// it falls back to all channels rather than going silent, because silence is
// the worst possible failure mode for a monitoring product.

func (s *Server) GetMonitorChannels(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	id := chi.URLParam(r, "id")

	// Tenant check first: without it, any authenticated user could read the
	// routing of another tenant's monitor by guessing an ID.
	var exists bool
	if err := s.DB.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM monitors WHERE id=$1 AND tenant_id=$2)`,
		id, c.TenantID).Scan(&exists); err != nil || !exists {
		writeErr(w, 404, "not found")
		return
	}

	rows, err := s.DB.Query(r.Context(),
		`SELECT channel_id::text FROM monitor_channels WHERE monitor_id=$1`, id)
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var cid string
		if rows.Scan(&cid) == nil {
			out = append(out, cid)
		}
	}
	writeJSON(w, 200, map[string]any{"channel_ids": out})
}

type routingReq struct {
	ChannelIDs []string `json:"channel_ids"`
}

// SetMonitorChannels replaces the routing wholesale rather than patching it.
// A partial update API for a set would need add/remove semantics and a way to
// express "none"; replacing the whole set keeps the client simple and makes
// the operation idempotent.
func (s *Server) SetMonitorChannels(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	id := chi.URLParam(r, "id")

	var in routingReq
	if err := decode(r, &in); err != nil {
		writeErr(w, 400, "bad request")
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "db")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var exists bool
	if err := tx.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM monitors WHERE id=$1 AND tenant_id=$2)`,
		id, c.TenantID).Scan(&exists); err != nil || !exists {
		writeErr(w, 404, "not found")
		return
	}

	if _, err := tx.Exec(r.Context(),
		`DELETE FROM monitor_channels WHERE monitor_id=$1`, id); err != nil {
		writeErr(w, 500, "db")
		return
	}

	for _, cid := range in.ChannelIDs {
		// The tenant filter is inside the INSERT, so a caller cannot route a
		// monitor to another tenant's channel by supplying its ID.
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO monitor_channels (monitor_id, channel_id)
			SELECT $1, id FROM alert_channels WHERE id=$2 AND tenant_id=$3`,
			id, cid, c.TenantID); err != nil {
			slog.Error("set monitor channel", "monitor", id, "channel", cid, "err", err)
			writeErr(w, 400, "invalid channel in selection")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "db")
		return
	}
	w.WriteHeader(204)
}
