// stripe-setup creates the Stripe products and prices for each paid plan and
// writes the price IDs back into the plans table.
//
// Doing this in code rather than clicking through the dashboard means the
// Stripe side of the configuration is reproducible: a new environment (or a
// colleague's sandbox) is one command away from matching.
//
// Safe to re-run: it looks up existing products by metadata before creating.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/price"
	"github.com/stripe/stripe-go/v81/product"

	"github.com/abrifuh6/buam-pulse/internal/config"
	"github.com/abrifuh6/buam-pulse/internal/db"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.StripeSecretKey == "" {
		log.Error("STRIPE_SECRET_KEY is required")
		os.Exit(1)
	}
	stripe.Key = cfg.StripeSecretKey

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `
		SELECT code, name, price_cents, price_cents_quarterly, price_cents_yearly
		FROM plans WHERE price_cents > 0 ORDER BY sort_order`)
	if err != nil {
		log.Error("read plans", "err", err)
		os.Exit(1)
	}
	type planRow struct {
		code, name                 string
		monthly, quarterly, yearly int64
	}
	var toCreate []planRow
	for rows.Next() {
		var p planRow
		if rows.Scan(&p.code, &p.name, &p.monthly, &p.quarterly, &p.yearly) == nil {
			toCreate = append(toCreate, p)
		}
	}
	rows.Close()

	// Stripe models a quarterly price as a monthly interval with a count of 3,
	// not as its own interval type.
	periods := []struct {
		name     string
		column   string
		interval stripe.PriceRecurringInterval
		count    int64
	}{
		{"monthly", "stripe_price_id", stripe.PriceRecurringIntervalMonth, 1},
		{"quarterly", "stripe_price_quarterly", stripe.PriceRecurringIntervalMonth, 3},
		{"yearly", "stripe_price_yearly", stripe.PriceRecurringIntervalYear, 1},
	}

	for _, p := range toCreate {
		var productID string
		search := product.Search(&stripe.ProductSearchParams{
			SearchParams: stripe.SearchParams{
				Query: "metadata['pulse_plan']:'" + p.code + "'",
			},
		})
		for search.Next() {
			productID = search.Product().ID
			break
		}

		if productID == "" {
			prod, err := product.New(&stripe.ProductParams{
				Name:        stripe.String("Pulse " + p.name),
				Description: stripe.String("Pulse uptime monitoring — " + p.name + " plan"),
				Metadata:    map[string]string{"pulse_plan": p.code},
			})
			if err != nil {
				log.Error("create product", "plan", p.code, "err", err)
				os.Exit(1)
			}
			productID = prod.ID
			log.Info("created product", "plan", p.code, "id", productID)
		}

		for _, period := range periods {
			var cents int64
			switch period.name {
			case "monthly":
				cents = p.monthly
			case "quarterly":
				cents = p.quarterly
			case "yearly":
				cents = p.yearly
			}
			if cents == 0 {
				continue
			}

			// Prices are immutable in Stripe: changing an amount means a new
			// price, not an edit. Reuse an active one that already matches.
			var priceID string
			pi := price.List(&stripe.PriceListParams{
				Product: stripe.String(productID),
				Active:  stripe.Bool(true),
			})
			for pi.Next() {
				pr := pi.Price()
				if pr.UnitAmount == cents && pr.Recurring != nil &&
					pr.Recurring.Interval == period.interval &&
					pr.Recurring.IntervalCount == period.count {
					priceID = pr.ID
					break
				}
			}

			if priceID == "" {
				pr, err := price.New(&stripe.PriceParams{
					Product:    stripe.String(productID),
					Currency:   stripe.String(string(stripe.CurrencyUSD)),
					UnitAmount: stripe.Int64(cents),
					Recurring: &stripe.PriceRecurringParams{
						Interval:      stripe.String(string(period.interval)),
						IntervalCount: stripe.Int64(period.count),
					},
					Metadata: map[string]string{
						"pulse_plan":   p.code,
						"pulse_period": period.name,
					},
				})
				if err != nil {
					log.Error("create price", "plan", p.code, "period", period.name, "err", err)
					os.Exit(1)
				}
				priceID = pr.ID
				log.Info("created price", "plan", p.code, "period", period.name, "cents", cents)
			}

			if _, err := pool.Exec(ctx,
				"UPDATE plans SET "+period.column+"=$2 WHERE code=$1", p.code, priceID); err != nil {
				log.Error("save price id", "plan", p.code, "err", err)
				os.Exit(1)
			}
		}
		log.Info("plan ready", "plan", p.code)
	}

	log.Info("stripe setup complete")
}
