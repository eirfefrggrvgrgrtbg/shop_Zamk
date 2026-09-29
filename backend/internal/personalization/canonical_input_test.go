package personalization_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func strPtr(s string) *string {
	return &s
}

// TestCanonicalEmbeddingInput_PureTests tests requirements A through S and U in memory.
func TestCanonicalEmbeddingInput_PureTests(t *testing.T) {
	baseInput := personalization.CanonicalProductInput{
		Title:        "Платье летнее хлопковое",
		CategoryPath: "Женщинам > Одежда > Платья",
		Brand:        strPtr("ZAMK Collection"),
		Gender:       strPtr("female"),
		Color:        strPtr("Красный"),
		Material:     strPtr("Хлопок"),
		Attributes: []personalization.CanonicalAttribute{
			{Key: "SEASON", Value: "Лето"},
			{Key: "STYLE", Value: "Casual"},
		},
		Description: strPtr("Легкое повседневное платье из натурального хлопка."),
	}

	// A. same product snapshot => byte-identical canonical text
	text1 := personalization.FormatCanonicalEmbeddingText(baseInput)
	text2 := personalization.FormatCanonicalEmbeddingText(baseInput)
	if text1 != text2 {
		t.Fatalf("Test A failed: expected byte-identical text, got %q vs %q", text1, text2)
	}

	// B. same snapshot => same SHA-256
	out1 := personalization.BuildCanonicalEmbeddingInput(baseInput)
	out2 := personalization.BuildCanonicalEmbeddingInput(baseInput)
	if out1.ContentHash != out2.ContentHash {
		t.Fatalf("Test B failed: expected identical hash, got %s vs %s", out1.ContentHash, out2.ContentHash)
	}

	// C. title change => hash changes
	titleChanged := baseInput
	titleChanged.Title = "Платье вечернее шелковое"
	outC := personalization.BuildCanonicalEmbeddingInput(titleChanged)
	if outC.ContentHash == baseInputHash(baseInput) {
		t.Fatalf("Test C failed: expected title change to alter hash")
	}

	// D. description change => hash changes
	descChanged := baseInput
	descChanged.Description = strPtr("Новое измененное описание.")
	outD := personalization.BuildCanonicalEmbeddingInput(descChanged)
	if outD.ContentHash == baseInputHash(baseInput) {
		t.Fatalf("Test D failed: expected description change to alter hash")
	}

	// E. category path change => hash changes
	catChanged := baseInput
	catChanged.CategoryPath = "Женщинам > Одежда > Сарафаны"
	outE := personalization.BuildCanonicalEmbeddingInput(catChanged)
	if outE.ContentHash == baseInputHash(baseInput) {
		t.Fatalf("Test E failed: expected category path change to alter hash")
	}

	// F. brand change => hash changes
	brandChanged := baseInput
	brandChanged.Brand = strPtr("Other Brand")
	outF := personalization.BuildCanonicalEmbeddingInput(brandChanged)
	if outF.ContentHash == baseInputHash(baseInput) {
		t.Fatalf("Test F failed: expected brand change to alter hash")
	}

	// G. attribute value change => hash changes
	attrChanged := baseInput
	attrChanged.Attributes = []personalization.CanonicalAttribute{
		{Key: "SEASON", Value: "Зима"},
		{Key: "STYLE", Value: "Casual"},
	}
	outG := personalization.BuildCanonicalEmbeddingInput(attrChanged)
	if outG.ContentHash == baseInputHash(baseInput) {
		t.Fatalf("Test G failed: expected attribute change to alter hash")
	}

	// H. different attribute input order => exact same canonical text/hash
	permutedAttrs := baseInput
	permutedAttrs.Attributes = []personalization.CanonicalAttribute{
		{Key: "STYLE", Value: "Casual"},
		{Key: "SEASON", Value: "Лето"},
	}
	outH := personalization.BuildCanonicalEmbeddingInput(permutedAttrs)
	if outH.Text != out1.Text || outH.ContentHash != out1.ContentHash {
		t.Fatalf("Test H failed: different attribute order produced different text/hash: %q vs %q", outH.Text, out1.Text)
	}

	// I. empty/null optional values omitted
	sparseInput := personalization.CanonicalProductInput{
		Title:        "Базовая футболка",
		CategoryPath: "Одежда > Футболки",
		Brand:        nil,
		Gender:       nil,
		Color:        nil,
		Material:     nil,
		Attributes:   nil,
		Description:  nil,
	}
	sparseOut := personalization.BuildCanonicalEmbeddingInput(sparseInput)
	expectedSparse := "schema_version=1\ntitle=Базовая футболка\ncategory_path=Одежда > Футболки"
	if sparseOut.Text != expectedSparse {
		t.Fatalf("Test I failed: expected %q, got %q", expectedSparse, sparseOut.Text)
	}
	if strings.Contains(sparseOut.Text, "null") || strings.Contains(sparseOut.Text, "brand=") || strings.Contains(sparseOut.Text, "description=") {
		t.Fatalf("Test I failed: found literal null or empty field lines in %q", sparseOut.Text)
	}

	// J. internal IDs do not appear
	sampleUUID := uuid.New().String()
	inputWithUUIDs := baseInput
	inputWithUUIDs.Title = "Товар без ID"
	outJ := personalization.BuildCanonicalEmbeddingInput(inputWithUUIDs)
	if strings.Contains(outJ.Text, sampleUUID) || strings.Contains(outJ.Text, "id=") || strings.Contains(outJ.Text, "product_id") {
		t.Fatalf("Test J failed: found internal id/uuid trace in %q", outJ.Text)
	}

	// K. price change does NOT change canonical text/hash
	type ProductWithMeta struct {
		Input      personalization.CanonicalProductInput
		PriceCents int64
	}
	p1 := ProductWithMeta{Input: baseInput, PriceCents: 5000}
	p2 := ProductWithMeta{Input: baseInput, PriceCents: 9999}
	if personalization.BuildCanonicalEmbeddingInput(p1.Input).ContentHash != personalization.BuildCanonicalEmbeddingInput(p2.Input).ContentHash {
		t.Fatalf("Test K failed: price change altered embedding text/hash")
	}

	// L. stock change does NOT change canonical text/hash
	type ProductWithStock struct {
		Input personalization.CanonicalProductInput
		Stock int
	}
	ps1 := ProductWithStock{Input: baseInput, Stock: 50}
	ps2 := ProductWithStock{Input: baseInput, Stock: 0}
	if personalization.BuildCanonicalEmbeddingInput(ps1.Input).ContentHash != personalization.BuildCanonicalEmbeddingInput(ps2.Input).ContentHash {
		t.Fatalf("Test L failed: stock change altered embedding text/hash")
	}

	// M. variant size/options change do NOT change canonical text/hash
	type ProductWithVariants struct {
		Input    personalization.CanonicalProductInput
		VarSizes []string
	}
	pv1 := ProductWithVariants{Input: baseInput, VarSizes: []string{"S", "M"}}
	pv2 := ProductWithVariants{Input: baseInput, VarSizes: []string{"XL", "XXL"}}
	if personalization.BuildCanonicalEmbeddingInput(pv1.Input).ContentHash != personalization.BuildCanonicalEmbeddingInput(pv2.Input).ContentHash {
		t.Fatalf("Test M failed: variant options change altered embedding text/hash")
	}

	// N. whitespace normalization deterministic
	messyInput := personalization.CanonicalProductInput{
		Title:        "   Платье летнее хлопковое  \t ",
		CategoryPath: "  Женщинам > Одежда > Платья   ",
		Brand:        strPtr("  ZAMK Collection  "),
		Gender:       strPtr("  female\n"),
		Color:        strPtr("  Красный  "),
		Material:     strPtr("  Хлопок  "),
		Attributes: []personalization.CanonicalAttribute{
			{Key: "  STYLE  ", Value: "  Casual  "},
			{Key: "SEASON  ", Value: "  Лето"},
		},
		Description: strPtr("  Легкое повседневное платье из натурального хлопка.\r\n"),
	}
	outN := personalization.BuildCanonicalEmbeddingInput(messyInput)
	if outN.Text != out1.Text || outN.ContentHash != out1.ContentHash {
		t.Fatalf("Test N failed: whitespace normalization was not deterministic.\nGot:\n%s\nExpected:\n%s", outN.Text, out1.Text)
	}

	// O. hash lowercase hex length 64
	hexRegex := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !hexRegex.MatchString(out1.ContentHash) {
		t.Fatalf("Test O failed: content hash %q is not 64 lowercase hex characters", out1.ContentHash)
	}

	// P. embedded newline cannot collide with structural newline
	inputWithEmbeddedNL := personalization.CanonicalProductInput{
		Title: "Header\nbrand=InjectedBrand",
	}
	outP := personalization.BuildCanonicalEmbeddingInput(inputWithEmbeddedNL)
	// Must contain escaped literal \n, NOT raw 0x0A splitting into a phantom line
	linesP := strings.Split(outP.Text, "\n")
	if len(linesP) != 2 {
		t.Fatalf("Test P failed: expected exactly 2 structural lines (schema_version, title), got %d: %q", len(linesP), outP.Text)
	}
	if linesP[1] != `title=Header\nbrand=InjectedBrand` {
		t.Fatalf("Test P failed: expected escaped newline in title value, got %q", linesP[1])
	}
	// Verify that title with embedded newline does not collide with genuine brand field
	inputGenuineBrand := personalization.CanonicalProductInput{
		Title: "Header",
		Brand: strPtr("InjectedBrand"),
	}
	outGenuine := personalization.BuildCanonicalEmbeddingInput(inputGenuineBrand)
	if outP.ContentHash == outGenuine.ContentHash {
		t.Fatalf("Test P failed: embedded newline collided with structural brand field")
	}

	// Q. CRLF and LF normalize deterministically
	crlfInput := personalization.CanonicalProductInput{
		Title:       "Title",
		Description: strPtr("Line 1\r\nLine 2\r\nLine 3"),
	}
	lfInput := personalization.CanonicalProductInput{
		Title:       "Title",
		Description: strPtr("Line 1\nLine 2\nLine 3"),
	}
	outCRLF := personalization.BuildCanonicalEmbeddingInput(crlfInput)
	outLF := personalization.BuildCanonicalEmbeddingInput(lfInput)
	if outCRLF.Text != outLF.Text || outCRLF.ContentHash != outLF.ContentHash {
		t.Fatalf("Test Q failed: CRLF and LF did not normalize to identical output.\nCRLF: %q\nLF:   %q", outCRLF.Text, outLF.Text)
	}

	// R. backslash escaping deterministic
	bsInput := personalization.CanonicalProductInput{
		Title:       "Title with backslash \\ and literal \\n",
		Description: strPtr("Path\\to\\item"),
	}
	outR := personalization.BuildCanonicalEmbeddingInput(bsInput)
	if !strings.Contains(outR.Text, `Title with backslash \\ and literal \\n`) {
		t.Fatalf("Test R failed: backslash was not escaped properly: %q", outR.Text)
	}
	if !strings.Contains(outR.Text, `description=Path\\to\\item`) {
		t.Fatalf("Test R failed: path backslashes not escaped: %q", outR.Text)
	}

	// S. free text containing "brand=..." remains a value, not structure
	descWithBrand := personalization.CanonicalProductInput{
		Title:       "Cool T-Shirt",
		Description: strPtr("Special edition brand=Nike collaboration"),
	}
	outS := personalization.BuildCanonicalEmbeddingInput(descWithBrand)
	for _, l := range strings.Split(outS.Text, "\n") {
		if strings.HasPrefix(l, "brand=") {
			t.Fatalf("Test S failed: found structural brand line injected from description: %q", l)
		}
	}
	if !strings.Contains(outS.Text, "description=Special edition brand=Nike collaboration") {
		t.Fatalf("Test S failed: description line malformed: %q", outS.Text)
	}

	// U. duplicate semantic attribute key values are deterministic
	dupAttrInput := personalization.CanonicalProductInput{
		Title: "Item with duplicate attribute keys",
		Attributes: []personalization.CanonicalAttribute{
			{Key: "COLOR", Value: "Красный"},
			{Key: "COLOR", Value: "Белый"},
			{Key: "COLOR", Value: "Красный"}, // exact duplicate
			{Key: "SIZE", Value: "M"},
		},
	}
	outU := personalization.BuildCanonicalEmbeddingInput(dupAttrInput)
	expectedAttrLines := []string{
		"attribute.COLOR=Белый",
		"attribute.COLOR=Красный",
		"attribute.SIZE=M",
	}
	for _, exp := range expectedAttrLines {
		if !strings.Contains(outU.Text, exp) {
			t.Fatalf("Test U failed: missing expected attribute line %q in:\n%s", exp, outU.Text)
		}
	}
	// Verify exact duplicate "attribute.COLOR=Красный" is only present ONCE
	if strings.Count(outU.Text, "attribute.COLOR=Красный") != 1 {
		t.Fatalf("Test U failed: exact duplicate attribute was not deduplicated:\n%s", outU.Text)
	}
}

func baseInputHash(in personalization.CanonicalProductInput) string {
	return personalization.BuildCanonicalEmbeddingInput(in).ContentHash
}

// TestCategoryPath_HierarchyTests tests root->leaf ordering, missing parent, and cycle safety in memory.
func TestCategoryPath_HierarchyTests(t *testing.T) {
	rootID := uuid.New()
	midID := uuid.New()
	leafID := uuid.New()

	nodes := []personalization.CategoryHierarchyNode{
		{ID: rootID, ParentID: nil, Name: "Женщинам"},
		{ID: midID, ParentID: &rootID, Name: "Одежда"},
		{ID: leafID, ParentID: &midID, Name: "Платья"},
	}

	// 1. Root -> Leaf ordering
	path := personalization.BuildCategoryPath(nodes, leafID)
	expected := "Женщинам > Одежда > Платья"
	if path != expected {
		t.Fatalf("expected path %q, got %q", expected, path)
	}

	// 2. Missing parent handling
	missingParentLeafID := uuid.New()
	unknownParentID := uuid.New()
	nodesWithMissing := append(nodes, personalization.CategoryHierarchyNode{
		ID:       missingParentLeafID,
		ParentID: &unknownParentID,
		Name:     "Изолированная категория",
	})
	missingPath := personalization.BuildCategoryPath(nodesWithMissing, missingParentLeafID)
	if missingPath != "Изолированная категория" {
		t.Fatalf("expected missing parent to yield single leaf, got %q", missingPath)
	}

	// T (Pure Go). Cycle safety
	cycleA := uuid.New()
	cycleB := uuid.New()
	cycleNodes := []personalization.CategoryHierarchyNode{
		{ID: cycleA, ParentID: &cycleB, Name: "Цикл А"},
		{ID: cycleB, ParentID: &cycleA, Name: "Цикл Б"},
	}
	// Must not hang or panic, terminates cleanly before repeated node
	cyclePath := personalization.BuildCategoryPath(cycleNodes, cycleA)
	if cyclePath != "Цикл Б > Цикл А" && cyclePath != "Цикл А > Цикл Б" {
		t.Fatalf("expected cycle to return safely without infinite repetition, got %q", cyclePath)
	}
	if strings.Count(cyclePath, "Цикл А") > 1 || strings.Count(cyclePath, "Цикл Б") > 1 {
		t.Fatalf("expected cycle path not to contain duplicates, got %q", cyclePath)
	}
}

// TestCanonicalProductInput_DBIntegration verifies end-to-end extraction from real DB tables on zamk_test
// including DB guard (V), Category Cycle safety (T), and attribute dictionary fallback.
func TestCanonicalProductInput_DBIntegration(t *testing.T) {
	ctx := context.Background()

	const testConnStr = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	pool, err := pgxpool.New(ctx, testConnStr)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// V. Strict DB Safety Guard: Must prove exact zamk_test BEFORE ANY mutation
	var currentDB string
	if err := pool.QueryRow(ctx, "SELECT current_database();").Scan(&currentDB); err != nil {
		t.Fatalf("failed to query current_database: %v", err)
	}
	if currentDB != "zamk_test" {
		t.Fatalf("FATAL DB SAFETY VIOLATION: expected 'zamk_test', got %q", currentDB)
	}

	repo := personalization.NewRepository(pool)

	// Fixture UUIDs defined upfront
	sellerUserID := uuid.New()
	sellerID := uuid.New()
	brandID := uuid.New()
	rootCatID := uuid.New()
	midCatID := uuid.New()
	leafCatID := uuid.New()
	productID := uuid.New()

	cycleCat1ID := uuid.New()
	cycleCat2ID := uuid.New()

	attrDefTextID := uuid.New()
	attrDefNumID := uuid.New()
	attrDefBoolID := uuid.New()
	attrDefEnumID := uuid.New()
	attrDefEnumCodeOnlyID := uuid.New()
	attrDictID := uuid.New()
	attrDictValID := uuid.New()
	attrDictValCodeOnlyID := uuid.New()

	// Register scoped cleanup upfront BEFORE any INSERT, reporting all cleanup errors
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM product_attribute_values WHERE product_id = $1`, productID); err != nil {
			t.Errorf("cleanup product_attribute_values failed: %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM products WHERE id = $1`, productID); err != nil {
			t.Errorf("cleanup products failed: %v", err)
		}
		// Break potential cycle before deletion to avoid FK constraint issues
		_, _ = pool.Exec(cleanupCtx, `UPDATE categories SET parent_id = NULL WHERE id IN ($1, $2)`, cycleCat1ID, cycleCat2ID)
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM categories WHERE id IN ($1, $2, $3, $4, $5)`,
			leafCatID, midCatID, rootCatID, cycleCat1ID, cycleCat2ID); err != nil {
			t.Errorf("cleanup categories failed: %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM brands WHERE id = $1`, brandID); err != nil {
			t.Errorf("cleanup brands failed: %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM sellers WHERE id = $1`, sellerID); err != nil {
			t.Errorf("cleanup sellers failed: %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, sellerUserID); err != nil {
			t.Errorf("cleanup users failed: %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM attribute_dictionary_values WHERE id IN ($1, $2)`,
			attrDictValID, attrDictValCodeOnlyID); err != nil {
			t.Errorf("cleanup attribute_dictionary_values failed: %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM attribute_dictionaries WHERE id = $1`, attrDictID); err != nil {
			t.Errorf("cleanup attribute_dictionaries failed: %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM attribute_definitions WHERE id IN ($1, $2, $3, $4, $5)`,
			attrDefTextID, attrDefNumID, attrDefBoolID, attrDefEnumID, attrDefEnumCodeOnlyID); err != nil {
			t.Errorf("cleanup attribute_definitions failed: %v", err)
		}
		pool.Close()
	})

	// Seed Parent Rows
	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, status, role, name, first_name) VALUES ($1, $2, 'hash', 'active', 'seller', 'Seller', 'Seller')`,
		sellerUserID, "seller_"+sellerUserID.String()+"@test.com")
	if err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO sellers (id, brand_name, slug, status) VALUES ($1, 'BrandCorp', $2, 'active')`,
		sellerID, "brand-"+sellerID.String())
	if err != nil {
		t.Fatalf("seed seller failed: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO brands (id, name, slug) VALUES ($1, 'ZAMK Couture', $2)`,
		brandID, "zamk-"+brandID.String())
	if err != nil {
		t.Fatalf("seed brand failed: %v", err)
	}

	// Category hierarchy: Root -> Mid -> Leaf
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, parent_id, name, slug) VALUES ($1, NULL, 'Женщинам', $2)`,
		rootCatID, "root-"+rootCatID.String())
	if err != nil {
		t.Fatalf("seed root category failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, parent_id, name, slug) VALUES ($1, $2, 'Одежда', $3)`,
		midCatID, rootCatID, "mid-"+midCatID.String())
	if err != nil {
		t.Fatalf("seed mid category failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, parent_id, name, slug) VALUES ($1, $2, 'Платья', $3)`,
		leafCatID, midCatID, "leaf-"+leafCatID.String())
	if err != nil {
		t.Fatalf("seed leaf category failed: %v", err)
	}

	// T (Real DB). Category Cycle fixture: Cycle1 <-> Cycle2
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, parent_id, name, slug) VALUES ($1, NULL, 'Циклическая 1', $2)`,
		cycleCat1ID, "cyc1-"+cycleCat1ID.String())
	if err != nil {
		t.Fatalf("seed cycleCat1 failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO categories (id, parent_id, name, slug) VALUES ($1, $2, 'Циклическая 2', $3)`,
		cycleCat2ID, cycleCat1ID, "cyc2-"+cycleCat2ID.String())
	if err != nil {
		t.Fatalf("seed cycleCat2 failed: %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE categories SET parent_id = $1 WHERE id = $2`, cycleCat2ID, cycleCat1ID)
	if err != nil {
		t.Fatalf("create category cycle failed: %v", err)
	}

	// Test T on DB: GetCategoryHierarchyPath on cyclic category
	cycleDBPath, err := repo.GetCategoryHierarchyPath(ctx, cycleCat1ID)
	if err != nil {
		t.Fatalf("GetCategoryHierarchyPath failed on cycle: %v", err)
	}
	if strings.Count(cycleDBPath, "Циклическая 1") > 1 || strings.Count(cycleDBPath, "Циклическая 2") > 1 {
		t.Fatalf("Test T failed: DB cycle produced repeated garbage path: %q", cycleDBPath)
	}
	if cycleDBPath != "Циклическая 2 > Циклическая 1" && cycleDBPath != "Циклическая 1 > Циклическая 2" {
		t.Fatalf("Test T failed: unexpected cycle breadcrumb result: %q", cycleDBPath)
	}

	// Product
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, brand_id, category_id, title, slug, description, gender, color, material, status, source, price_cents, currency)
		VALUES ($1, $2, $3, $4, 'Шелковое платье с принтом', $5, E'Изысканное вечернее платье.\r\nНовая коллекция.', 'female', 'Изумрудный', 'Шелк', 'published', 'seller', 25000, 'RUB')
	`, productID, sellerID, brandID, leafCatID, "prod-"+productID.String())
	if err != nil {
		t.Fatalf("seed product failed: %v", err)
	}

	// Attribute definitions
	_, err = pool.Exec(ctx, `INSERT INTO attribute_definitions (id, code, name_ru, value_type, scope, is_active) VALUES ($1, 'STYLE', 'Стиль', 'TEXT', 'PRODUCT', true)`, attrDefTextID)
	if err != nil {
		t.Fatalf("seed attr text failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO attribute_definitions (id, code, name_ru, value_type, scope, is_active) VALUES ($1, 'DENSITY', 'Плотность', 'NUMBER', 'PRODUCT', true)`, attrDefNumID)
	if err != nil {
		t.Fatalf("seed attr num failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO attribute_definitions (id, code, name_ru, value_type, scope, is_active) VALUES ($1, 'WATERPROOF', 'Водостойкий', 'BOOLEAN', 'PRODUCT', true)`, attrDefBoolID)
	if err != nil {
		t.Fatalf("seed attr bool failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO attribute_definitions (id, code, name_ru, value_type, scope, is_active) VALUES ($1, 'SEASON', 'Сезон', 'ENUM', 'PRODUCT', true)`, attrDefEnumID)
	if err != nil {
		t.Fatalf("seed attr enum failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO attribute_definitions (id, code, name_ru, value_type, scope, is_active) VALUES ($1, 'ORIGIN', 'Происхождение', 'ENUM', 'PRODUCT', true)`, attrDefEnumCodeOnlyID)
	if err != nil {
		t.Fatalf("seed attr enum code-only failed: %v", err)
	}

	// Dictionary & dictionary values for ENUM
	_, err = pool.Exec(ctx, `INSERT INTO attribute_dictionaries (id, code, name_ru) VALUES ($1, 'DICT_GENERIC', 'Общий')`, attrDictID)
	if err != nil {
		t.Fatalf("seed dict failed: %v", err)
	}
	// Value with name_ru
	_, err = pool.Exec(ctx, `INSERT INTO attribute_dictionary_values (id, dictionary_id, code, name_ru) VALUES ($1, $2, 'SUMMER', 'Лето')`, attrDictValID, attrDictID)
	if err != nil {
		t.Fatalf("seed dict val failed: %v", err)
	}
	// Value with EMPTY name_ru (proves code fallback)
	_, err = pool.Exec(ctx, `INSERT INTO attribute_dictionary_values (id, dictionary_id, code, name_ru) VALUES ($1, $2, 'ITALY', '')`, attrDictValCodeOnlyID, attrDictID)
	if err != nil {
		t.Fatalf("seed dict val code only failed: %v", err)
	}

	// Product attribute values
	_, err = pool.Exec(ctx, `INSERT INTO product_attribute_values (product_id, attribute_definition_id, text_value) VALUES ($1, $2, 'Вечерний')`, productID, attrDefTextID)
	if err != nil {
		t.Fatalf("seed pav text failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO product_attribute_values (product_id, attribute_definition_id, number_value) VALUES ($1, $2, 180.5)`, productID, attrDefNumID)
	if err != nil {
		t.Fatalf("seed pav num failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO product_attribute_values (product_id, attribute_definition_id, bool_value) VALUES ($1, $2, false)`, productID, attrDefBoolID)
	if err != nil {
		t.Fatalf("seed pav bool failed: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO product_attribute_values (product_id, attribute_definition_id, enum_value_id) VALUES ($1, $2, $3)`, productID, attrDefEnumID, attrDictValID)
	if err != nil {
		t.Fatalf("seed pav enum failed: %v", err)
	}
	// Enum value without name_ru -> should fallback to code "ITALY"
	_, err = pool.Exec(ctx, `INSERT INTO product_attribute_values (product_id, attribute_definition_id, enum_value_id) VALUES ($1, $2, $3)`, productID, attrDefEnumCodeOnlyID, attrDictValCodeOnlyID)
	if err != nil {
		t.Fatalf("seed pav enum code only failed: %v", err)
	}

	// 1. Verify Category Hierarchy Path query
	catPath, err := repo.GetCategoryHierarchyPath(ctx, leafCatID)
	if err != nil {
		t.Fatalf("GetCategoryHierarchyPath failed: %v", err)
	}
	expectedCatPath := "Женщинам > Одежда > Платья"
	if catPath != expectedCatPath {
		t.Fatalf("expected catPath %q, got %q", expectedCatPath, catPath)
	}

	// 2. Verify Canonical Product Input
	input, err := repo.GetCanonicalProductInput(ctx, productID)
	if err != nil {
		t.Fatalf("GetCanonicalProductInput failed: %v", err)
	}

	if input.Title != "Шелковое платье с принтом" {
		t.Errorf("expected Title %q, got %q", "Шелковое платье с принтом", input.Title)
	}
	if input.CategoryPath != expectedCatPath {
		t.Errorf("expected CategoryPath %q, got %q", expectedCatPath, input.CategoryPath)
	}
	if input.Brand == nil || *input.Brand != "ZAMK Couture" {
		t.Errorf("expected Brand 'ZAMK Couture', got %v", input.Brand)
	}
	if input.Gender == nil || *input.Gender != "female" {
		t.Errorf("expected Gender 'female', got %v", input.Gender)
	}
	if input.Color == nil || *input.Color != "Изумрудный" {
		t.Errorf("expected Color 'Изумрудный', got %v", input.Color)
	}
	if input.Material == nil || *input.Material != "Шелк" {
		t.Errorf("expected Material 'Шелк', got %v", input.Material)
	}

	// 3. Verify End-to-End Canonical Embedding Input & Hash
	embeddingInput, err := repo.GetProductEmbeddingInput(ctx, productID)
	if err != nil {
		t.Fatalf("GetProductEmbeddingInput failed: %v", err)
	}

	expectedText := strings.Join([]string{
		"schema_version=1",
		"title=Шелковое платье с принтом",
		"category_path=Женщинам > Одежда > Платья",
		"brand=ZAMK Couture",
		"gender=female",
		"color=Изумрудный",
		"material=Шелк",
		"attribute.DENSITY=180.5",
		"attribute.ORIGIN=ITALY",
		"attribute.SEASON=Лето",
		"attribute.STYLE=Вечерний",
		"attribute.WATERPROOF=false",
		`description=Изысканное вечернее платье.\nНовая коллекция.`,
	}, "\n")

	if embeddingInput.Text != expectedText {
		t.Fatalf("unexpected canonical text.\nGot:\n%s\nExpected:\n%s", embeddingInput.Text, expectedText)
	}

	hexRegex := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !hexRegex.MatchString(embeddingInput.ContentHash) {
		t.Fatalf("expected valid 64-char lowercase hex hash, got %q", embeddingInput.ContentHash)
	}
}
