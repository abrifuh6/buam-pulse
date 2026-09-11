// Package plans enforces per-tenant subscription limits.
//
// Every check runs inside the caller's transaction where possible, because
// "count then insert" is a race: two concurrent requests can both see 2
// monitors under a limit of 3 and both insert. Taking the count with the
// tenant row locked closes that window.
package plans

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Limits struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	MaxMonitors    int    `json:"max_monitors"`
	MinInterval    int    `json:"min_interval"`
	MaxMembers     int    `json:"max_members"`
	MaxChannels    int    `json:"max_channels"`
	RetentionDays  int    `json:"retention_days"`
	PriceCents     int    `json:"price_cents"`
	PriceQuarterly *int   `json:"price_cents_quarterly"`
	PriceYearly    *int   `json:"price_cents_yearly"`
}

type Usage struct {
	Monitors int `json:"monitors"`
	Members  int `json:"members"`
	Channels int `json:"channels"`
}

// Querier covers both *pgxpool.Pool and pgx.Tx so callers can reuse their
// transaction rather than opening a second connection.
type Querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func Get(ctx context.Context, q Querier, tenantID string) (Limits, error) {
	var l Limits
	err := q.QueryRow(ctx, `
		SELECT p.code, p.name, p.max_monitors, p.min_interval,
		       p.max_members, p.max_channels, p.retention_days, p.price_cents,
		       p.price_cents_quarterly, p.price_cents_yearly
		FROM tenants t JOIN plans p ON p.code = t.plan_code
		WHERE t.id = $1`, tenantID).
		Scan(&l.Code, &l.Name, &l.MaxMonitors, &l.MinInterval,
			&l.MaxMembers, &l.MaxChannels, &l.RetentionDays, &l.PriceCents,
			&l.PriceQuarterly, &l.PriceYearly)
	return l, err
}

func GetUsage(ctx context.Context, q Querier, tenantID string) (Usage, error) {
	var u Usage
	err := q.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM monitors       WHERE tenant_id=$1),
		       (SELECT count(*) FROM users          WHERE tenant_id=$1),
		       (SELECT count(*) FROM alert_channels WHERE tenant_id=$1)`, tenantID).
		Scan(&u.Monitors, &u.Members, &u.Channels)
	return u, err
}

// LimitError is returned when an action would exceed the plan. Handlers turn
// it into a 402 Payment Required, which is more informative than a 403: it
// tells the client the action is legitimate but the plan is the obstacle.
type LimitError struct {
	Resource string
	Limit    int
	Plan     string
}

func (e LimitError) Error() string {
	return fmt.Sprintf("your %s plan allows %d %s — upgrade to add more",
		e.Plan, e.Limit, e.Resource)
}

// CheckMonitor validates both count and interval before a monitor is created.
// tx must already hold a lock on the tenant row (see LockTenant).
func CheckMonitor(ctx context.Context, tx pgx.Tx, tenantID string, intervalSeconds int) error {
	l, err := Get(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	if intervalSeconds < l.MinInterval {
		return fmt.Errorf("your %s plan allows checks no more often than every %d seconds",
			l.Name, l.MinInterval)
	}
	var count int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM monitors WHERE tenant_id=$1`, tenantID).Scan(&count); err != nil {
		return err
	}
	if count >= l.MaxMonitors {
		return LimitError{Resource: "monitors", Limit: l.MaxMonitors, Plan: l.Name}
	}
	return nil
}

func CheckMember(ctx context.Context, tx pgx.Tx, tenantID string) error {
	l, err := Get(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	// Pending invitations count against the limit: otherwise a tenant could
	// send 100 invites on a 5-seat plan and end up over it as they accept.
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM users WHERE tenant_id=$1)
		     + (SELECT count(*) FROM invitations
		        WHERE tenant_id=$1 AND accepted_at IS NULL AND expires_at > now())`,
		tenantID).Scan(&count); err != nil {
		return err
	}
	if count >= l.MaxMembers {
		return LimitError{Resource: "team members", Limit: l.MaxMembers, Plan: l.Name}
	}
	return nil
}

func CheckChannel(ctx context.Context, tx pgx.Tx, tenantID string) error {
	l, err := Get(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	var count int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM alert_channels WHERE tenant_id=$1`, tenantID).Scan(&count); err != nil {
		return err
	}
	if count >= l.MaxChannels {
		return LimitError{Resource: "alert channels", Limit: l.MaxChannels, Plan: l.Name}
	}
	return nil
}

// LockTenant serialises limit checks for one tenant. Concurrent requests from
// the same tenant queue behind each other; different tenants are unaffected.
func LockTenant(ctx context.Context, tx pgx.Tx, tenantID string) error {
	var id string
	return tx.QueryRow(ctx, `SELECT id FROM tenants WHERE id=$1 FOR UPDATE`, tenantID).Scan(&id)
}
