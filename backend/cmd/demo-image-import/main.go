// DEMO.D1.2 — import of prepared demo product photos into the persistent demo catalog (zamk_demo, bucket zamk-demo).
//
//	backend/demo/demo.sh images [--manifest <file>] [--dry-run] [--allow-partial]
//
// Every photo goes through the regular media pipeline, exactly as Seller Studio + Admin would do it:
// stage (storage service, byte validation, upload) → 4:5 crop with rendition → gallery revision of the published product
// (continueSelling) → approval by the demo admin (file check + moderation checks of the resulting product).
// Product ids, variants, SKUs, prices, stock and statuses are never touched; nothing is deleted.
//
// Safety and idempotency:
//   - refuses any database but a local zamk_demo (URL and current_database()) and any bucket but zamk-demo;
//   - the whole manifest is validated first (entries, provenance, products, colours, files, format, size): if any
//     file is missing or invalid the command lists them and changes nothing (--allow-partial imports only the
//     products whose files are all valid);
//   - the staging id of a photo is derived from product, colour and file content: a re-run finds the photo already
//     in the gallery and does nothing (no new rows, no new objects).
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/catalog"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/google/uuid"
)

const (
	demoDB     = "zamk_demo"
	demoBucket = "zamk-demo"
	minCropW   = 800 // the pipeline's minimum 4:5 crop
	minCropH   = 1000
)

var clientNS = uuid.MustParse("de000000-0000-4000-8000-0000000d1d12")

type entry struct {
	ProductSlug   string `json:"productSlug"`
	ProductTitle  string `json:"productTitle"`
	ColorCode     string `json:"colorCode"`
	ColorName     string `json:"colorName"`
	ColorID       string `json:"colorId"`
	File          string `json:"file"`
	IsMain        bool   `json:"isMain"`
	Origin        string `json:"origin"`
	Source        string `json:"source"`
	License       string `json:"license"`
	SyntheticDemo bool   `json:"syntheticDemo"`
}

type manifest struct {
	Version  int     `json:"version"`
	Database string  `json:"database"`
	Bucket   string  `json:"bucket"`
	Images   []entry `json:"images"`
}

type photo struct {
	entry
	path      string
	productID uuid.UUID
	colorID   uuid.UUID
	sha       string
	bytes     []byte
	mime      string
}

func fail(code int, format string, a ...any) {
	fmt.Fprintf(os.Stderr, "DEMO IMAGES: "+format+"\n", a...)
	os.Exit(code)
}

func main() {
	manifestPath := flag.String("manifest", "", "manifest file (default: backend/demo/images/manifest.json)")
	dryRun := flag.Bool("dry-run", false, "validate and print the plan, change nothing")
	allowPartial := flag.Bool("allow-partial", false, "import the products whose files are all present and valid, list the rest")
	flag.Parse()
	if *manifestPath == "" {
		fail(2, "--manifest is required")
	}

	// ---- environment guards (before any connection that could write)
	dsn := os.Getenv("DEMO_DATABASE_URL")
	u, err := url.Parse(dsn)
	if err != nil || dsn == "" {
		fail(2, "REFUSED: DEMO_DATABASE_URL is missing or invalid")
	}
	if name := strings.TrimPrefix(u.Path, "/"); name != demoDB {
		fail(2, "REFUSED: database %q is not %s", name, demoDB)
	}
	if h := u.Hostname(); h != "localhost" && h != "127.0.0.1" && h != "::1" {
		fail(2, "REFUSED: host %q is not local", h)
	}
	cfg, err := config.Load()
	if err != nil {
		fail(2, "config: %v", err)
	}
	if cfg.S3.Bucket != demoBucket {
		fail(2, "REFUSED: S3 bucket %q is not %s", cfg.S3.Bucket, demoBucket)
	}

	m, err := loadManifest(*manifestPath)
	if err != nil {
		fail(2, "manifest: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pg, err := postgres.NewClient(ctx, dsn)
	if err != nil {
		fail(1, "connect: %v", err)
	}
	defer pg.Close()
	var cur string
	if err := pg.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&cur); err != nil || cur != demoDB {
		fail(2, "REFUSED: connected database is %q, not %s", cur, demoDB)
	}

	// ---- validate everything before the first write
	byProduct, problems := resolve(ctx, pg, m, filepath.Dir(*manifestPath), int64(cfg.S3.UploadMaxSizeMB))
	ready := map[string][]*photo{}
	for slug, ph := range byProduct {
		if len(problems[slug]) == 0 {
			ready[slug] = ph
		}
	}
	if n := countProblems(problems); n > 0 {
		fmt.Fprintf(os.Stderr, "%d problem(s) in %d product(s):\n", n, len(problems))
		for _, slug := range sortedKeys(problems) {
			for _, p := range problems[slug] {
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", slug, p)
			}
		}
		if !*allowPartial {
			fail(3, "nothing imported (complete the files above, or use --allow-partial)")
		}
	}
	if *dryRun {
		fmt.Printf("dry-run: %d product(s) ready with %d photo(s); nothing changed\n", len(ready), countPhotos(ready))
		return
	}
	if len(ready) == 0 {
		fail(3, "no product has a complete, valid set of photos; nothing imported")
	}

	// ---- the regular pipeline
	provider, err := storage.NewS3Client(&cfg.S3)
	if err != nil {
		fail(1, "S3: %v", err)
	}
	productsRepo := products.NewRepository(pg.Pool)
	sellersRepo := sellers.NewRepository(pg.Pool)
	storageSvc := storage.NewService(provider, productsRepo, catalog.NewRepository(pg.Pool), sellersRepo, pg)
	productSvc := products.NewService(productsRepo, sellersRepo, pg, nil, nil).WithMediaObjectVerifier(provider.ObjectExists)
	var adminID uuid.UUID
	adminEmail := envOr("DEMO_ADMIN_EMAIL", "admin@demo.zamk.test")
	if err := pg.Pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1 AND role = 'admin'`, adminEmail).Scan(&adminID); err != nil {
		fail(1, "demo admin %s: %v", adminEmail, err)
	}

	added, unchanged := 0, 0
	for _, slug := range sortedKeys(ready) {
		n, err := importProduct(ctx, pg, storageSvc, productSvc, adminID, ready[slug])
		if err != nil {
			fail(1, "%s: %v (products before it are imported; a re-run continues safely)", slug, err)
		}
		if n == 0 {
			unchanged++
		} else {
			added += n
			fmt.Printf("  %s: +%d photo(s)\n", slug, n)
		}
	}
	fmt.Printf("demo images on %s/%s: +%d photo(s) in %d product(s), %d product(s) already complete\n", cur, demoBucket, added, len(ready)-unchanged, unchanged)
}

func loadManifest(path string) (*manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Version != 1 || m.Database != demoDB || m.Bucket != demoBucket {
		return nil, fmt.Errorf("expected version 1 for %s/%s", demoDB, demoBucket)
	}
	if len(m.Images) == 0 {
		return nil, errors.New("no images")
	}
	return &m, nil
}

func validProvenance(e entry) string {
	switch e.Origin {
	case "synthetic":
		if !e.SyntheticDemo {
			return "origin synthetic requires syntheticDemo: true"
		}
	case "own", "licensed":
		if e.SyntheticDemo {
			return "syntheticDemo: true requires origin synthetic"
		}
	default:
		return fmt.Sprintf("origin must be synthetic, own or licensed (got %q)", e.Origin)
	}
	if strings.TrimSpace(e.Source) == "" || strings.TrimSpace(e.License) == "" {
		return "source and license must be filled"
	}
	return ""
}

// resolve checks every entry against the manifest rules, the catalog and the file system; it reads only.
func resolve(ctx context.Context, pg *postgres.Client, m *manifest, baseDir string, maxMB int64) (map[string][]*photo, map[string][]string) {
	byProduct := map[string][]*photo{}
	problems := map[string][]string{}
	add := func(slug, format string, a ...any) {
		problems[slug] = append(problems[slug], fmt.Sprintf(format, a...))
	}
	seen := map[string]bool{}
	mains := map[string]int{}
	if maxMB <= 0 {
		maxMB = 10
	}
	for _, e := range m.Images {
		e := e
		key := e.ProductSlug + "/" + e.ColorCode
		if seen[key] {
			add(e.ProductSlug, "duplicate entry for colour %s", e.ColorCode)
			continue
		}
		seen[key] = true
		if e.IsMain {
			mains[e.ProductSlug]++
		}
		ph := &photo{entry: e}
		byProduct[e.ProductSlug] = append(byProduct[e.ProductSlug], ph)
		if p := validProvenance(e); p != "" {
			add(e.ProductSlug, "%s: %s", e.ColorCode, p)
		}
		if err := pg.Pool.QueryRow(ctx, `SELECT id FROM products WHERE slug = $1`, e.ProductSlug).Scan(&ph.productID); err != nil {
			add(e.ProductSlug, "product not found")
			continue
		}
		if err := pg.Pool.QueryRow(ctx, `SELECT c.id FROM colors c WHERE c.code = $1
			AND EXISTS (SELECT 1 FROM product_variants v WHERE v.product_id = $2 AND v.color_id = c.id AND v.is_active)`, e.ColorCode, ph.productID).Scan(&ph.colorID); err != nil {
			add(e.ProductSlug, "%s: not an active colour of the product", e.ColorCode)
			continue
		}
		if e.ColorID != "" && e.ColorID != ph.colorID.String() {
			add(e.ProductSlug, "%s: colorId %s does not match the catalog (%s)", e.ColorCode, e.ColorID, ph.colorID)
		}
		if filepath.IsAbs(e.File) || strings.Contains(e.File, "..") {
			add(e.ProductSlug, "%s: file path must be relative to the manifest", e.ColorCode)
			continue
		}
		ph.path = filepath.Join(baseDir, e.File)
		b, err := os.ReadFile(ph.path)
		if err != nil {
			add(e.ProductSlug, "%s: missing file %s", e.ColorCode, ph.path)
			continue
		}
		if int64(len(b)) > maxMB*1024*1024 {
			add(e.ProductSlug, "%s: file larger than %d MB", e.ColorCode, maxMB)
			continue
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			add(e.ProductSlug, "%s: not a readable image (%v)", e.ColorCode, err)
			continue
		}
		mime := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[format]
		if mime == "" {
			add(e.ProductSlug, "%s: format %s is not JPEG, PNG or WebP", e.ColorCode, format)
			continue
		}
		cw, ch := float64(cfg.Width), float64(cfg.Height)
		if cw/ch > 0.8 {
			cw = ch * 0.8
		} else {
			ch = cw / 0.8
		}
		if cw < minCropW || ch < minCropH {
			add(e.ProductSlug, "%s: %dx%d is too small (its 4:5 crop must be at least %dx%d)", e.ColorCode, cfg.Width, cfg.Height, minCropW, minCropH)
			continue
		}
		sum := sha256.Sum256(b)
		ph.sha, ph.bytes, ph.mime = hex.EncodeToString(sum[:]), b, mime
	}
	for slug := range byProduct {
		if mains[slug] != 1 {
			add(slug, "exactly one isMain photo is required (found %d)", mains[slug])
		}
	}
	return byProduct, problems
}

// importProduct stages and crops the product's missing photos, then submits the full gallery as a revision of the
// published product and approves it. Returns the number of photos added (0 = already complete, nothing written).
func importProduct(ctx context.Context, pg *postgres.Client, st *storage.Service, ps *products.Service, adminID uuid.UUID, photos []*photo) (int, error) {
	pid := photos[0].productID
	var sellerUserID uuid.UUID
	if err := pg.Pool.QueryRow(ctx, `SELECT su.user_id FROM seller_users su JOIN products p ON p.seller_id = su.seller_id
		WHERE p.id = $1 AND su.role = 'owner' ORDER BY su.created_at LIMIT 1`, pid).Scan(&sellerUserID); err != nil {
		return 0, fmt.Errorf("seller owner: %w", err)
	}
	type canon struct {
		id     uuid.UUID
		color  *uuid.UUID
		isMain bool
		sort   int
	}
	var gallery []canon
	rows, err := pg.Pool.Query(ctx, `SELECT id, color_id, is_main, sort_order FROM product_images WHERE product_id = $1 ORDER BY sort_order, created_at`, pid)
	if err != nil {
		return 0, err
	}
	inGallery := map[uuid.UUID]bool{}
	for rows.Next() {
		var c canon
		if err := rows.Scan(&c.id, &c.color, &c.isMain, &c.sort); err != nil {
			rows.Close()
			return 0, err
		}
		gallery = append(gallery, c)
		inGallery[c.id] = true
	}
	rows.Close()

	var mainID uuid.UUID
	ids := map[*photo]uuid.UUID{}
	added := 0
	for _, ph := range photos {
		clientID := uuid.NewMD5(clientNS, []byte(ph.ProductSlug+"/"+ph.ColorCode+"/"+ph.sha))
		var existing uuid.UUID
		err := pg.Pool.QueryRow(ctx, `SELECT s.id FROM product_media_staging s JOIN product_images i ON i.id = s.id
			WHERE s.product_id = $1 AND s.client_media_id = $2`, pid, clientID).Scan(&existing)
		if err == nil && inGallery[existing] {
			ids[ph] = existing // already imported: no upload, no staging, no write
		} else {
			resp, err := st.StageSellerProductImage(ctx, sellerUserID, pid, clientID, bytes.NewReader(ph.bytes), filepath.Base(ph.path), int64(len(ph.bytes)), ph.mime, 10)
			if err != nil {
				return 0, fmt.Errorf("stage %s: %w", ph.ColorCode, err)
			}
			if _, err := st.AutoCropSellerProductImage(ctx, sellerUserID, pid, resp.StagedMediaID); err != nil {
				return 0, fmt.Errorf("4:5 crop %s: %w", ph.ColorCode, err)
			}
			ids[ph] = resp.StagedMediaID
			added++
		}
		if ph.IsMain {
			mainID = ids[ph]
		}
	}
	currentMain := uuid.Nil
	for _, c := range gallery {
		if c.isMain {
			currentMain = c.id
		}
	}
	if added == 0 && currentMain == mainID {
		return 0, nil
	}

	// full target gallery: the manifest photos (main first, then by colour) followed by any other existing photo
	var reqs []products.ProductImageRequest
	push := func(id uuid.UUID, color *uuid.UUID, alt string) {
		id, main, order := id, id == mainID, len(reqs)
		var a *string
		if alt != "" {
			a = &alt
		}
		reqs = append(reqs, products.ProductImageRequest{ID: &id, ImageURL: "x", ColorID: color, IsMain: &main, SortOrder: &order, AltText: a})
	}
	ordered := append([]*photo{}, photos...)
	sort.SliceStable(ordered, func(a, b int) bool { return ordered[a].IsMain && !ordered[b].IsMain })
	used := map[uuid.UUID]bool{}
	for _, ph := range ordered {
		c := ph.colorID
		push(ids[ph], &c, ph.ProductTitle+" — "+strings.ToLower(ph.ColorName))
		used[ids[ph]] = true
	}
	for _, c := range gallery {
		if !used[c.id] {
			push(c.id, c.color, "")
		}
	}
	yes := true
	if _, err := ps.UpdateProductForSeller(ctx, sellerUserID, pid, products.UpdateProductRequest{ContinueSelling: &yes, Images: reqs}); err != nil {
		return 0, fmt.Errorf("gallery revision: %w", err)
	}
	comment := "DEMO.D1.2 photo import"
	if err := ps.ApproveProduct(ctx, adminID, pid, &comment); err != nil {
		return 0, fmt.Errorf("approve gallery: %w", err)
	}
	// verify: every photo canonical with its 4:5 rendition, the main one set
	for _, ph := range photos {
		var rendition *string
		var isMain bool
		if err := pg.Pool.QueryRow(ctx, `SELECT rendition_url, is_main FROM product_images WHERE id = $1 AND product_id = $2`, ids[ph], pid).Scan(&rendition, &isMain); err != nil {
			return 0, fmt.Errorf("verify %s: %w", ph.ColorCode, err)
		}
		if rendition == nil || *rendition == "" || isMain != ph.IsMain {
			return 0, fmt.Errorf("verify %s: rendition or main flag missing", ph.ColorCode)
		}
	}
	return added, nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func countProblems(p map[string][]string) (n int) {
	for _, v := range p {
		n += len(v)
	}
	return
}

func countPhotos(p map[string][]*photo) (n int) {
	for _, v := range p {
		n += len(v)
	}
	return
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
