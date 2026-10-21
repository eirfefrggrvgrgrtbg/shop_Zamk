// DEMO.D1 — idempotent seed of the isolated demo environment (database zamk_demo, bucket zamk-demo).
//
//	DEMO_DATABASE_URL=postgres://…/zamk_demo DEMO_ACCOUNT_PASSWORD=… go run ./cmd/demo-seed
//
// Safety: the seed never reads the regular .env/config. It refuses any database other than zamk_demo (checked on the URL
// and again with current_database() before the first write) and any host that is not local. It only inserts/updates
// rows it owns (deterministic ids / natural keys); stock rows are created once and never overwritten.
// Account passwords come from DEMO_ACCOUNT_PASSWORD and are only written when an account is created
// (DEMO_RESET_PASSWORDS=1 re-hashes them).
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/staff"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	demoDB     = "zamk_demo"
	demoBucket = "zamk-demo"
	emailHost  = "demo.zamk.test"
)

var ns = uuid.MustParse("de000000-0000-4000-8000-00000000d1d1")

func id(kind, key string) uuid.UUID { return uuid.NewMD5(ns, []byte(kind+":"+key)) }

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "DEMO SEED REFUSED/FAILED: "+format+"\n", a...)
	os.Exit(1)
}

func assertDemoDSN(dsn string) {
	u, err := url.Parse(dsn)
	if err != nil || dsn == "" {
		fail("DEMO_DATABASE_URL is missing or not a valid URL")
	}
	if name := strings.TrimPrefix(u.Path, "/"); name != demoDB {
		fail("database %q is not %s", name, demoDB)
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
	default:
		fail("host %q is not local", u.Hostname())
	}
}

type sellerDef struct{ slug, name, desc, email string }
type brandDef struct {
	slug, name, desc, seller string
	primary                  bool
}

var sellers = []sellerDef{
	{"nord-atelier", "Nord Atelier", "Трикотаж и худи свободного кроя из плотного футера.", "seller.nord@" + emailHost},
	{"urban-soles", "Urban Soles", "Городские кроссовки на каждый день.", "seller.soles@" + emailHost},
	{"carry-studio", "Carry Studio", "Сумки и шопперы из плотного хлопка и кожи.", "seller.carry@" + emailHost},
}

var brands = []brandDef{
	{"nord-atelier", "Nord Atelier", "Скандинавская сдержанность и плотные ткани.", "nord-atelier", true},
	{"linea-9", "Linea 9", "Базовые вещи в приглушённой палитре.", "nord-atelier", false},
	{"urban-soles", "Urban Soles", "Кроссовки для города.", "urban-soles", true},
	{"carry-studio", "Carry Studio", "Аксессуары для ежедневных маршрутов.", "carry-studio", true},
}

type productDef struct {
	title, slug, category, brand, gender, material string
	price, oldPrice                                int64
	colors                                         []string // colors.code
	sizes                                          []string // size_values.value (in the category's system)
	status                                         string
	soldOut                                        map[string]bool // "COLOR/size": no stock
}

var hoodieSizes = []string{"XS", "S", "M", "L", "XL"}
var shoeSizes = []string{"38", "39", "40", "41", "42", "43", "44"}

func products() []productDef {
	var ps []productDef
	hood := []struct {
		t, s, b string
		price   int64
		cols    []string
	}{
		{"Худи оверсайз «Фьорд»", "hoodie-fjord", "nord-atelier", 790000, []string{"BLACK", "GREY"}},
		{"Худи на молнии «Берген»", "hoodie-bergen", "nord-atelier", 840000, []string{"BEIGE", "GREEN"}},
		{"Худи базовое «Осло»", "hoodie-oslo", "nord-atelier", 690000, []string{"GREY", "BLUE", "BLACK"}},
		{"Худи укороченное «Тромсё»", "hoodie-tromso", "nord-atelier", 720000, []string{"PINK", "BEIGE"}},
		{"Худи с капюшоном «Лофотен»", "hoodie-lofoten", "nord-atelier", 910000, []string{"GREEN", "BLACK"}},
		{"Худи Linea «Серый меланж»", "hoodie-linea-melange", "linea-9", 590000, []string{"GREY"}},
		{"Худи Linea «Тёмно-синее»", "hoodie-linea-navy", "linea-9", 610000, []string{"BLUE", "BLACK"}},
		{"Худи Linea «Песочное»", "hoodie-linea-sand", "linea-9", 630000, []string{"BEIGE", "BROWN"}},
	}
	for i, h := range hood {
		p := productDef{title: h.t, slug: h.s, category: "hoodies", brand: h.b, gender: "unisex", material: "Хлопок 80%, полиэстер 20%",
			price: h.price, colors: h.cols, sizes: hoodieSizes, status: "published", soldOut: map[string]bool{}}
		if i%3 == 0 {
			p.oldPrice = h.price + 150000
		}
		if i == 0 { // a size that exists but is not in stock
			p.soldOut["BLACK/XS"], p.soldOut["GREY/XL"] = true, true
		}
		ps = append(ps, p)
	}
	shoes := []struct {
		t, s  string
		price int64
		cols  []string
		sz    []string
	}{
		{"Кроссовки «Курьер»", "sneakers-courier", 1190000, []string{"WHITE", "BLACK"}, shoeSizes},
		{"Кроссовки «Бульвар»", "sneakers-boulevard", 1290000, []string{"WHITE", "BEIGE"}, shoeSizes[1:6]},
		{"Кроссовки «Пассаж»", "sneakers-passage", 1350000, []string{"BLACK"}, shoeSizes},
		{"Кроссовки «Набережная»", "sneakers-embankment", 1240000, []string{"GREY", "WHITE"}, shoeSizes[:5]},
		{"Кроссовки «Мост»", "sneakers-bridge", 1420000, []string{"GREEN", "BLACK"}, shoeSizes[2:]},
		{"Кроссовки «Аллея»", "sneakers-alley", 1150000, []string{"WHITE"}, shoeSizes},
		{"Кроссовки «Рассвет»", "sneakers-dawn", 1380000, []string{"PINK", "WHITE"}, shoeSizes[:4]},
		{"Кроссовки «Перекрёсток»", "sneakers-crossing", 1320000, []string{"BROWN", "BEIGE"}, shoeSizes[1:]},
		{"Кроссовки «Эстакада»", "sneakers-overpass", 1260000, []string{"BLUE", "BLACK"}, shoeSizes},
	}
	for i, s := range shoes {
		p := productDef{title: s.t, slug: s.s, category: "sneakers", brand: "urban-soles", gender: "unisex", material: "Натуральная кожа, текстиль",
			price: s.price, colors: s.cols, sizes: s.sz, status: "published", soldOut: map[string]bool{}}
		if i%4 == 1 {
			p.oldPrice = s.price + 200000
		}
		if i == 0 {
			p.soldOut["WHITE/38"], p.soldOut["BLACK/44"] = true, true
		}
		ps = append(ps, p)
	}
	bags := []struct {
		t, s  string
		price int64
		cols  []string
	}{
		{"Шоппер «Базар»", "bag-bazaar", 390000, []string{"BEIGE", "BLACK"}},
		{"Сумка через плечо «Маршрут»", "bag-route", 890000, []string{"BROWN", "BLACK"}},
		{"Сумка-тоут «Офис»", "bag-office", 1190000, []string{"BLACK"}},
		{"Поясная сумка «Прогулка»", "bag-walk", 490000, []string{"GREEN", "GREY"}},
		{"Мини-сумка «Вечер»", "bag-evening", 790000, []string{"RED", "BLACK"}},
		{"Сумка хобо «Лето»", "bag-summer", 990000, []string{"YELLOW", "BEIGE"}},
		{"Шоппер «Рынок»", "bag-market", 350000, []string{"GREEN", "BEIGE"}},
	}
	for i, b := range bags {
		p := productDef{title: b.t, slug: b.s, category: "bags", brand: "carry-studio", gender: "unisex", material: "Плотный хлопок, натуральная кожа",
			price: b.price, colors: b.cols, sizes: []string{"ONE_SIZE"}, status: "published", soldOut: map[string]bool{}}
		if i == 2 {
			p.oldPrice = b.price + 250000
		}
		ps = append(ps, p)
	}
	// lifecycle examples for the seller / admin cabinets (not visible in the shop)
	ps = append(ps,
		productDef{title: "Худи «Черновик»", slug: "hoodie-draft", category: "hoodies", brand: "nord-atelier", gender: "unisex", material: "Хлопок 100%", price: 700000, colors: []string{"BLACK"}, sizes: []string{"M", "L"}, status: "draft"},
		productDef{title: "Кроссовки «На модерации»", slug: "sneakers-pending", category: "sneakers", brand: "urban-soles", gender: "unisex", material: "Текстиль", price: 1100000, colors: []string{"WHITE"}, sizes: []string{"41", "42"}, status: "pending_moderation"},
	)
	return ps
}

func main() {
	dsn := os.Getenv("DEMO_DATABASE_URL")
	assertDemoDSN(dsn)
	password := os.Getenv("DEMO_ACCOUNT_PASSWORD")
	if len(password) < 12 {
		fail("DEMO_ACCOUNT_PASSWORD (at least 12 characters) must be provided in the environment; it is never stored in git")
	}
	resetPw := os.Getenv("DEMO_RESET_PASSWORDS") == "1"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pg, err := postgres.NewClient(ctx, dsn)
	if err != nil {
		fail("connect: %v", err)
	}
	defer pg.Close()
	var cur string
	if err := pg.Pool.QueryRow(ctx, `select current_database()`).Scan(&cur); err != nil || cur != demoDB {
		fail("connected database is %q, not %s", cur, demoDB)
	}

	var adminID uuid.UUID
	err = pg.RunInTx(ctx, func(tx pgx.Tx) error {
		// accounts
		adminID, err = upsertUser(ctx, tx, "admin", "Demo Admin", "admin@"+emailHost, "admin", password, resetPw)
		if err != nil {
			return err
		}
		if _, err = upsertUser(ctx, tx, "customer", "Demo Customer", "customer@"+emailHost, "customer", password, resetPw); err != nil {
			return err
		}
		sellerIDs := map[string]uuid.UUID{}
		for _, s := range sellers {
			uid, err := upsertUser(ctx, tx, "user-"+s.slug, s.name+" (owner)", s.email, "seller", password, resetPw)
			if err != nil {
				return err
			}
			sid := id("seller", s.slug)
			if err := tx.QueryRow(ctx, `
				INSERT INTO sellers (id, brand_name, slug, description, contact_email, status)
				VALUES ($1,$2,$3,$4,$5,'active')
				ON CONFLICT (slug) DO UPDATE SET brand_name=EXCLUDED.brand_name, description=EXCLUDED.description, contact_email=EXCLUDED.contact_email, status='active', updated_at=now()
				RETURNING id`, sid, s.name, s.slug, s.desc, s.email).Scan(&sid); err != nil {
				return fmt.Errorf("seller %s: %w", s.slug, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO seller_users (id, seller_id, user_id, role) VALUES ($1,$2,$3,'owner') ON CONFLICT (user_id) DO NOTHING`, id("seller_user", s.slug), sid, uid); err != nil {
				return fmt.Errorf("seller_user %s: %w", s.slug, err)
			}
			sellerIDs[s.slug] = sid
		}
		brandIDs := map[string]uuid.UUID{}
		for _, b := range brands {
			bid := id("brand", b.slug)
			if err := tx.QueryRow(ctx, `
				INSERT INTO brands (id, name, slug, description, is_active) VALUES ($1,$2,$3,$4,true)
				ON CONFLICT (slug) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, is_active=true, updated_at=now()
				RETURNING id`, bid, b.name, b.slug, b.desc).Scan(&bid); err != nil {
				return fmt.Errorf("brand %s: %w", b.slug, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO seller_brands (id, seller_id, brand_id, status, is_primary) VALUES ($1,$2,$3,'active',$4) ON CONFLICT (seller_id, brand_id) DO NOTHING`,
				id("seller_brand", b.slug), sellerIDs[b.seller], bid, b.primary); err != nil {
				return fmt.Errorf("seller_brand %s: %w", b.slug, err)
			}
			brandIDs[b.slug] = bid
		}
		// "Сумки" are one-size items: allow the One Size system for that category (the migrations map no system to it)
		if _, err := tx.Exec(ctx, `
			INSERT INTO category_size_systems (category_id, size_system_id, is_default, sort_order)
			SELECT c.id, s.id, true, 10 FROM categories c, size_systems s WHERE c.slug='bags' AND s.code='ONE_SIZE'
			ON CONFLICT DO NOTHING`); err != nil {
			return fmt.Errorf("bags size system: %w", err)
		}

		brandSeller := map[string]string{}
		for _, b := range brands {
			brandSeller[b.slug] = b.seller
		}
		for _, p := range products() {
			if err := upsertProduct(ctx, tx, p, sellerIDs[brandSeller[p.brand]], brandIDs[p.brand], adminID); err != nil {
				return fmt.Errorf("product %s: %w", p.slug, err)
			}
		}
		return nil
	})
	if err != nil {
		fail("%v", err)
	}
	if err := staff.NewRepository(pg.Pool).EnsureOwnerForSeed(ctx, adminID); err != nil {
		fail("admin staff role: %v", err)
	}
	if err := ensureBucket(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: bucket %s not prepared: %v\n", demoBucket, err)
	}

	var np, nv, nc int
	_ = pg.Pool.QueryRow(ctx, `select count(*) from products where slug in (select unnest($1::text[]))`, slugs()).Scan(&np)
	_ = pg.Pool.QueryRow(ctx, `select count(*) from product_variants where product_id in (select id from products where slug = any($1::text[]))`, slugs()).Scan(&nv)
	_ = pg.Pool.QueryRow(ctx, `select count(distinct category_id) from products where slug = any($1::text[])`, slugs()).Scan(&nc)
	fmt.Printf("demo seed ready on %s: %d products, %d variants, %d categories, %d sellers, %d brands\n", cur, np, nv, nc, len(sellers), len(brands))
}

func slugs() []string {
	var out []string
	for _, p := range products() {
		out = append(out, p.slug)
	}
	return out
}

func upsertUser(ctx context.Context, tx pgx.Tx, key, name, email, role, password string, reset bool) (uuid.UUID, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}
	setHash := "users.password_hash"
	if reset {
		setHash = "EXCLUDED.password_hash"
	}
	var uid uuid.UUID
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO users (id, name, email, password_hash, role, status, must_change_password)
		VALUES ($1,$2,$3,$4,$5,'active',false)
		ON CONFLICT (email) DO UPDATE SET name=EXCLUDED.name, role=EXCLUDED.role, status='active', password_hash=%s, updated_at=now()
		RETURNING id`, setHash), id("user", key), name, email, hash, role).Scan(&uid)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user %s: %w", email, err)
	}
	return uid, nil
}

func upsertProduct(ctx context.Context, tx pgx.Tx, p productDef, sellerID, brandID, adminID uuid.UUID) error {
	var catID uuid.UUID
	if err := tx.QueryRow(ctx, `select id from categories where slug=$1`, p.category).Scan(&catID); err != nil {
		return fmt.Errorf("category %s: %w", p.category, err)
	}
	var old *int64
	if p.oldPrice > 0 {
		old = &p.oldPrice
	}
	desc := fmt.Sprintf("%s. Материал: %s. Демонстрационный товар ZAMK.", p.title, p.material)
	published := p.status == "published"
	pid := id("product", p.slug)
	err := tx.QueryRow(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, title, slug, description, status, gender, material, care_instructions,
			price_cents, old_price_cents, currency, source, submitted_at, approved_at, published_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'Деликатная стирка при 30°C',$11,$12,'RUB','seller',
			CASE WHEN $8 <> 'draft' THEN now() END,
			CASE WHEN $13 THEN now() END,
			CASE WHEN $13 THEN now() END)
		ON CONFLICT (slug) DO UPDATE SET title=EXCLUDED.title, description=EXCLUDED.description, category_id=EXCLUDED.category_id,
			brand_id=EXCLUDED.brand_id, gender=EXCLUDED.gender, material=EXCLUDED.material, price_cents=EXCLUDED.price_cents,
			old_price_cents=EXCLUDED.old_price_cents, updated_at=now()
		RETURNING id`, pid, sellerID, catID, brandID, p.title, p.slug, desc, p.status, p.gender, p.material, p.price, old, published).Scan(&pid)
	if err != nil {
		return err
	}
	sysCode := "INT"
	switch p.category {
	case "sneakers":
		sysCode = "EU"
	case "bags":
		sysCode = "ONE_SIZE"
	}
	for _, color := range p.colors {
		for _, size := range p.sizes {
			var colorID, sizeID uuid.UUID
			if err := tx.QueryRow(ctx, `select id from colors where code=$1`, color).Scan(&colorID); err != nil {
				return fmt.Errorf("color %s: %w", color, err)
			}
			if err := tx.QueryRow(ctx, `select sv.id from size_values sv join size_systems ss on ss.id=sv.size_system_id where ss.code=$1 and sv.value=$2`, sysCode, size).Scan(&sizeID); err != nil {
				return fmt.Errorf("size %s/%s: %w", sysCode, size, err)
			}
			key := p.slug + "/" + color + "/" + size
			vid := id("variant", key)
			sku := strings.ToUpper(fmt.Sprintf("DEMO-%s-%s-%s", strings.ReplaceAll(p.slug, "-", ""), color, size))
			if len(sku) > 60 {
				sku = sku[:60]
			}
			if err := tx.QueryRow(ctx, `
				INSERT INTO product_variants (id, product_id, color_id, size_value_id, seller_sku, barcode, price_cents, is_active)
				VALUES ($1,$2,$3,$4,$5,$6,$7,true)
				ON CONFLICT (id) DO UPDATE SET price_cents=EXCLUDED.price_cents, is_active=true, updated_at=now()
				RETURNING id`, vid, pid, colorID, sizeID, sku, "DEMO-"+vid.String()[:13], p.price).Scan(&vid); err != nil {
				return fmt.Errorf("variant %s: %w", key, err)
			}
			if !published {
				continue
			}
			stock := 4 + (len(key)*7)%9 // 4..12, stable per variant
			if p.soldOut[color+"/"+size] {
				stock = 0
			}
			iid := id("inventory", key)
			tag, err := tx.Exec(ctx, `INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock, reserved_stock) VALUES ($1,$2,$3,$4,$5,0) ON CONFLICT (product_variant_id) DO NOTHING`, iid, pid, vid, sellerID, stock)
			if err != nil {
				return fmt.Errorf("inventory %s: %w", key, err)
			}
			if tag.RowsAffected() == 1 && stock > 0 { // movement only together with the newly created stock row
				if _, err := tx.Exec(ctx, `INSERT INTO stock_movements (id, inventory_item_id, product_id, product_variant_id, seller_id, type, quantity, reason, actor_user_id) VALUES ($1,$2,$3,$4,$5,'receipt',$6,'Demo seed stock',$7) ON CONFLICT (id) DO NOTHING`,
					id("movement", key), iid, pid, vid, sellerID, stock, adminID); err != nil {
					return fmt.Errorf("movement %s: %w", key, err)
				}
			}
		}
	}
	return nil
}

func ensureBucket(ctx context.Context) error {
	endpoint, access, secret := os.Getenv("DEMO_S3_ENDPOINT"), os.Getenv("DEMO_S3_ACCESS_KEY"), os.Getenv("DEMO_S3_SECRET_KEY")
	if endpoint == "" || access == "" || secret == "" {
		return fmt.Errorf("DEMO_S3_ENDPOINT/ACCESS_KEY/SECRET_KEY not set")
	}
	cl, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(access, secret, ""), Secure: false})
	if err != nil {
		return err
	}
	ok, err := cl.BucketExists(ctx, demoBucket)
	if err != nil {
		return err
	}
	if !ok {
		if err := cl.MakeBucket(ctx, demoBucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	policy := `{"Version":"2012-10-17","Statement":[{"Action":["s3:GetObject"],"Effect":"Allow","Principal":{"AWS":["*"]},"Resource":["arn:aws:s3:::` + demoBucket + `/*"]}]}`
	return cl.SetBucketPolicy(ctx, demoBucket, policy)
}
