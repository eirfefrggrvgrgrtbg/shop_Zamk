// DEMO.D1.1 — idempotent completion of the persistent demo catalog (database zamk_demo only).
//
//	DEMO_DATABASE_URL=postgres://…/zamk_demo go run ./cmd/demo-catalog-fill   (or: backend/demo/demo.sh fill)
//
// For an explicit list of demo products it ADDS what the publication rules require and the demo lacks:
//   - hoodies: material composition (100%) and a size chart XS–XL (CHEST, LENGTH, SLEEVE, cm);
//   - sneakers: a size chart (FOOT_LENGTH, cm) for exactly the EU sizes the product's variants offer;
//   - bags: nothing (the category requires neither composition nor a size chart).
//
// Safety: refuses any database other than zamk_demo (URL and current_database()) and non-local hosts; touches only the
// listed slugs; never updates or deletes an existing row (composition only when the product has none, size chart rows
// only for sizes that are missing); one transaction. Products, variants, SKUs, prices, stock and statuses are not touched.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/jackc/pgx/v5"
)

const demoDB = "zamk_demo"

var (
	hoodies  = []string{"hoodie-bergen", "hoodie-fjord", "hoodie-linea-melange", "hoodie-linea-navy", "hoodie-linea-sand", "hoodie-lofoten", "hoodie-oslo", "hoodie-tromso"}
	sneakers = []string{"sneakers-alley", "sneakers-boulevard", "sneakers-bridge", "sneakers-courier", "sneakers-crossing", "sneakers-dawn", "sneakers-embankment", "sneakers-overpass", "sneakers-passage"}

	// hoodie composition, matching the product material text "Хлопок 80%, полиэстер 20%"
	hoodieComposition = map[string]float64{"COTTON": 80, "POLYESTER": 20}
	// garment measurements of a relaxed hoodie, cm
	hoodieChart = map[string]map[string]float64{
		"XS": {"CHEST": 98, "LENGTH": 66, "SLEEVE": 60},
		"S":  {"CHEST": 104, "LENGTH": 68, "SLEEVE": 61},
		"M":  {"CHEST": 110, "LENGTH": 70, "SLEEVE": 62},
		"L":  {"CHEST": 116, "LENGTH": 72, "SLEEVE": 63},
		"XL": {"CHEST": 122, "LENGTH": 74, "SLEEVE": 64},
	}
	// insole (foot) length per EU size, cm
	footLength = map[string]float64{"38": 24.0, "39": 24.7, "40": 25.3, "41": 26.0, "42": 26.7, "43": 27.3, "44": 28.0}
)

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "DEMO FILL REFUSED/FAILED: "+format+"\n", a...)
	os.Exit(1)
}

func main() {
	dsn := os.Getenv("DEMO_DATABASE_URL")
	u, err := url.Parse(dsn)
	if err != nil || dsn == "" {
		fail("DEMO_DATABASE_URL is missing or not a valid URL")
	}
	if name := strings.TrimPrefix(u.Path, "/"); name != demoDB {
		fail("database %q is not %s", name, demoDB)
	}
	if h := u.Hostname(); h != "localhost" && h != "127.0.0.1" && h != "::1" {
		fail("host %q is not local", h)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pg, err := postgres.NewClient(ctx, dsn)
	if err != nil {
		fail("connect: %v", err)
	}
	defer pg.Close()
	var cur string
	if err := pg.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&cur); err != nil || cur != demoDB {
		fail("connected database is %q, not %s", cur, demoDB)
	}

	var compRows, charts, chartRows int
	err = pg.RunInTx(ctx, func(tx pgx.Tx) error {
		for _, slug := range hoodies {
			pid, catID, err := product(ctx, tx, slug, "hoodies")
			if err != nil {
				return err
			}
			var has bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM product_material_composition WHERE product_id = $1)`, pid).Scan(&has); err != nil {
				return err
			}
			if !has {
				for code, pct := range hoodieComposition {
					tag, err := tx.Exec(ctx, `INSERT INTO product_material_composition (product_id, material_id, percentage)
						SELECT $1, id, $3 FROM materials WHERE code = $2 AND is_active`, pid, code, pct)
					if err != nil {
						return fmt.Errorf("%s composition: %w", slug, err)
					}
					if tag.RowsAffected() != 1 {
						return fmt.Errorf("%s composition: material %s not found", slug, code)
					}
					compRows++
				}
			}
			c, r, err := fillChart(ctx, tx, pid, catID, func(size string) (map[string]float64, bool) { m, ok := hoodieChart[size]; return m, ok })
			if err != nil {
				return fmt.Errorf("%s size chart: %w", slug, err)
			}
			charts, chartRows = charts+c, chartRows+r
		}
		for _, slug := range sneakers {
			pid, catID, err := product(ctx, tx, slug, "sneakers")
			if err != nil {
				return err
			}
			c, r, err := fillChart(ctx, tx, pid, catID, func(size string) (map[string]float64, bool) {
				v, ok := footLength[size]
				return map[string]float64{"FOOT_LENGTH": v}, ok
			})
			if err != nil {
				return fmt.Errorf("%s size chart: %w", slug, err)
			}
			charts, chartRows = charts+c, chartRows+r
		}
		return nil
	})
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("demo catalog fill on %s: +%d composition rows, +%d size charts, +%d size chart rows (existing rows untouched)\n", cur, compRows, charts, chartRows)
}

// product resolves a listed demo product and checks it is the expected published demo item of that category.
func product(ctx context.Context, tx pgx.Tx, slug, category string) (string, string, error) {
	var pid, catID, catSlug, status string
	err := tx.QueryRow(ctx, `SELECT p.id::text, c.id::text, c.slug, p.status FROM products p JOIN categories c ON c.id = p.category_id WHERE p.slug = $1`, slug).Scan(&pid, &catID, &catSlug, &status)
	if err != nil {
		return "", "", fmt.Errorf("demo product %s: %w", slug, err)
	}
	if catSlug != category || status != "published" {
		return "", "", fmt.Errorf("demo product %s is %s/%s, expected published %s", slug, catSlug, status, category)
	}
	return pid, catID, nil
}

// fillChart creates the product's size chart if missing and adds rows for the sizes its active variants offer that the
// chart lacks. Existing charts and rows are never changed.
func fillChart(ctx context.Context, tx pgx.Tx, pid, catID string, measures func(size string) (map[string]float64, bool)) (int, int, error) {
	created := 0
	var chartID string
	err := tx.QueryRow(ctx, `SELECT id::text FROM product_size_charts WHERE product_id = $1`, pid).Scan(&chartID)
	if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx, `INSERT INTO product_size_charts (id, product_id, category_id) VALUES (gen_random_uuid(), $1, $2) RETURNING id::text`, pid, catID).Scan(&chartID); err != nil {
			return 0, 0, err
		}
		created = 1
	} else if err != nil {
		return 0, 0, err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT sv.id::text, sv.value FROM product_variants v JOIN size_values sv ON sv.id = v.size_value_id
		WHERE v.product_id = $1 AND v.is_active
		  AND NOT EXISTS (SELECT 1 FROM product_size_chart_rows r WHERE r.size_chart_id = $2 AND r.size_value_id = sv.id)`, pid, chartID)
	if err != nil {
		return 0, 0, err
	}
	type size struct{ id, value string }
	var missing []size
	for rows.Next() {
		var s size
		if err := rows.Scan(&s.id, &s.value); err != nil {
			rows.Close()
			return 0, 0, err
		}
		missing = append(missing, s)
	}
	rows.Close()
	added := 0
	for _, s := range missing {
		m, ok := measures(s.value)
		if !ok {
			return 0, 0, fmt.Errorf("no demo measurements for size %s", s.value)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO product_size_chart_rows (size_chart_id, size_value_id, measurements) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, chartID, s.id, m); err != nil {
			return 0, 0, err
		}
		added++
	}
	return created, added, nil
}
