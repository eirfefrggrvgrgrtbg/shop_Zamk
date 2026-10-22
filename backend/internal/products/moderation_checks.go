package products

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/observability"
	"github.com/google/uuid"
)

// CATALOG.C5.4: the moderation checks are evaluated on the FINAL product, i.e. the live product with the pending
// revision applied. The same function feeds the Admin workplace (Product.ModerationChecks) and the approval of a
// published product's revision, so the UI and the backend cannot disagree.

// Moderation check codes (the Admin UI owns the wording).
const (
	CheckMainImage          = "main_image"
	CheckDescription        = "description"
	CheckCategory           = "category"
	CheckBrand              = "brand"
	CheckActiveVariants     = "active_variants"
	CheckPrice              = "price"
	CheckSKUUnique          = "sku_unique"
	CheckRequiredAttributes = "required_attributes"
	CheckSizeChart          = "size_chart"
)

// ErrModerationChecksFailed: the product as it would be after approval does not pass the moderation checks.
var ErrModerationChecksFailed = errors.New("the resulting product does not pass the moderation checks")

type ModerationCheck struct {
	Code string `json:"code"`
	Pass bool   `json:"pass"`
}

// ModerationChecksError lists the failed checks of a refused approval (Revision: of a published product's revision,
// otherwise of a first publication).
type ModerationChecksError struct {
	Failed   []string
	Revision bool
}

func (e *ModerationChecksError) Error() string {
	return fmt.Sprintf("%v: %s", ErrModerationChecksFailed, strings.Join(e.Failed, ", "))
}
func (e *ModerationChecksError) Unwrap() []error {
	if e.Revision {
		return []error{ErrModerationChecksFailed, ErrRevisionNotApplicable}
	}
	return []error{ErrModerationChecksFailed}
}

// enforceModerationChecks refuses a publication whose resulting product fails the moderation checks. It is the one
// gate used by every approval/publication path (first publication and revisions), before anything is written.
func (s *Service) enforceModerationChecks(ctx context.Context, adminUserID uuid.UUID, live *Product, rev *PendingRevisionView, revision bool) error {
	checks, err := s.moderationChecks(ctx, finalProduct(live, rev))
	if err != nil {
		return err
	}
	failed := failedChecks(checks)
	if len(failed) == 0 {
		return nil
	}
	cerr := &ModerationChecksError{Failed: failed, Revision: revision}
	if revision {
		s.emitRevisionBlocked(ctx, adminUserID, live.ID, cerr)
	} else {
		observability.EmitBusinessEvent(ctx, s.logger, observability.BusinessEvent{
			EventName: "product.publish_blocked",
			Domain:    "product",
			Action:    "publish",
			Result:    "blocked",
			ActorID:   adminUserID.String(),
			ActorRole: "admin",
			Attributes: []slog.Attr{
				slog.String("product_id", live.ID.String()),
				slog.String("failed_checks", strings.Join(failed, ",")),
			},
		})
	}
	return cerr
}

// finalProduct is the product as approval would leave it: live data with every part the revision changes replaced.
func finalProduct(live *Product, rev *PendingRevisionView) *Product {
	f := *live
	if rev == nil {
		return &f
	}
	overlayRevisionScalars(&f, rev.scalars)
	if rev.VariantsChanged {
		f.Variants = overlayRevisionVariants(live.Variants, rev.Variants)
	}
	if rev.ImagesChanged {
		f.Images = rev.Images
		f.MainImageURL, f.MainImageObjectKey = mainImageOf(rev.Images, nil, nil)
		if f.MainImageURL == nil && len(rev.Images) > 0 {
			u := rev.Images[0].ImageURL
			f.MainImageURL = &u
		}
	}
	if rev.AttributesChanged {
		f.Attributes = rev.Attributes
	}
	if rev.MaterialCompositionChanged {
		f.MaterialComposition = rev.MaterialComposition
	}
	if rev.SizeChartChanged {
		if len(rev.SizeChartRows) == 0 {
			f.SizeChart = nil
		} else {
			chart := ProductSizeChart{ProductID: live.ID, Rows: rev.SizeChartRows}
			if live.SizeChart != nil {
				chart.ID = live.SizeChart.ID
			}
			if f.CategoryID != nil {
				chart.CategoryID = *f.CategoryID
			}
			f.SizeChart = &chart
		}
	}
	return &f
}

// moderationChecks evaluates the publication rules on a (final) product.
func (s *Service) moderationChecks(ctx context.Context, p *Product) ([]ModerationCheck, error) {
	var activeSKUs []string
	active := 0
	for _, v := range p.Variants {
		if !v.IsActive {
			continue
		}
		active++
		switch {
		case v.SellerSKU != nil && strings.TrimSpace(*v.SellerSKU) != "":
			activeSKUs = append(activeSKUs, strings.ToLower(strings.TrimSpace(*v.SellerSKU)))
		case v.SKU != nil && strings.TrimSpace(*v.SKU) != "":
			activeSKUs = append(activeSKUs, strings.ToLower(strings.TrimSpace(*v.SKU)))
		}
	}
	seen := map[string]bool{}
	unique := true
	for _, sku := range activeSKUs {
		if seen[sku] {
			unique = false
		}
		seen[sku] = true
	}
	desc := ""
	if p.Description != nil {
		desc = strings.TrimSpace(*p.Description)
	}
	hasImage := len(p.Images) > 0 || (p.MainImageURL != nil && *p.MainImageURL != "")

	requiredOK, sizeChartOK := true, true
	if p.CategoryID != nil {
		var err error
		if requiredOK, err = s.requiredAttributesPresent(ctx, *p.CategoryID, p); err != nil {
			return nil, err
		}
		var chartRequired bool
		if err := s.dbPool.Pool.QueryRow(ctx, `SELECT size_chart_required FROM categories WHERE id = $1`, *p.CategoryID).Scan(&chartRequired); err != nil {
			return nil, err
		}
		sizeChartOK = !chartRequired || (p.SizeChart != nil && len(p.SizeChart.Rows) > 0)
	}

	return []ModerationCheck{
		{CheckMainImage, hasImage},
		{CheckDescription, utf8.RuneCountInString(desc) > 10},
		{CheckCategory, p.CategoryID != nil},
		{CheckBrand, p.BrandID != nil},
		{CheckActiveVariants, active > 0},
		{CheckPrice, p.PriceCents > 0},
		{CheckSKUUnique, unique},
		{CheckRequiredAttributes, requiredOK},
		{CheckSizeChart, sizeChartOK},
	}, nil
}

// requiredAttributesPresent: every required PRODUCT-scope characteristic of the category has a value; a required
// composition must add up to 100%. Variant-scope axes (colour, size) are covered by the active variants check.
func (s *Service) requiredAttributesPresent(ctx context.Context, categoryID uuid.UUID, p *Product) (bool, error) {
	rows, err := s.dbPool.Pool.Query(ctx, `
		SELECT ad.id, ad.value_source
		FROM category_attribute_definitions cad
		JOIN attribute_definitions ad ON ad.id = cad.attribute_definition_id
		WHERE cad.category_id = $1 AND cad.required AND ad.is_active AND ad.scope = 'PRODUCT'`, categoryID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	have := map[uuid.UUID]bool{}
	for _, a := range p.Attributes {
		if a.EnumValueID != nil || (a.TextValue != nil && strings.TrimSpace(*a.TextValue) != "") || a.NumberValue != nil || a.BoolValue != nil {
			have[a.AttributeDefinitionID] = true
		}
	}
	var total float64
	for _, c := range p.MaterialComposition {
		total += c.Percentage
	}
	ok := true
	for rows.Next() {
		var id uuid.UUID
		var source string
		if err := rows.Scan(&id, &source); err != nil {
			return false, err
		}
		if source == "MATERIAL_COMPOSITION" {
			if total < 99.9 || total > 100.1 {
				ok = false
			}
		} else if !have[id] {
			ok = false
		}
	}
	return ok, rows.Err()
}

func failedChecks(checks []ModerationCheck) []string {
	var out []string
	for _, c := range checks {
		if !c.Pass {
			out = append(out, c.Code)
		}
	}
	return out
}

// RevisionAttributeChange is one characteristic as Admin reads it: its name with the live and the proposed value.
type RevisionAttributeChange struct {
	Name     string  `json:"name"`
	Current  *string `json:"current,omitempty"`
	Proposed *string `json:"proposed,omitempty"`
}

// revisionAttributeChanges lists the characteristics whose proposed value differs from the live one, with names.
func (s *Service) revisionAttributeChanges(ctx context.Context, live, proposed []ProductAttributeValue) ([]RevisionAttributeChange, error) {
	label := func(a ProductAttributeValue) (*string, error) {
		var v string
		switch {
		case a.EnumValueID != nil:
			if err := s.dbPool.Pool.QueryRow(ctx, `SELECT name_ru FROM attribute_dictionary_values WHERE id = $1`, *a.EnumValueID).Scan(&v); err != nil {
				return nil, err
			}
		case a.TextValue != nil:
			v = *a.TextValue
		case a.NumberValue != nil:
			v = fmt.Sprintf("%g", *a.NumberValue)
		case a.BoolValue != nil:
			v = map[bool]string{true: "Да", false: "Нет"}[*a.BoolValue]
		default:
			return nil, nil
		}
		return &v, nil
	}
	values := func(list []ProductAttributeValue) (map[uuid.UUID]*string, error) {
		out := map[uuid.UUID]*string{}
		for _, a := range list {
			l, err := label(a)
			if err != nil {
				return nil, err
			}
			out[a.AttributeDefinitionID] = l
		}
		return out, nil
	}
	cur, err := values(live)
	if err != nil {
		return nil, err
	}
	next, err := values(proposed)
	if err != nil {
		return nil, err
	}
	ids := map[uuid.UUID]bool{}
	for id := range cur {
		ids[id] = true
	}
	for id := range next {
		ids[id] = true
	}
	var out []RevisionAttributeChange
	for id := range ids {
		c, n := cur[id], next[id]
		if (c == nil && n == nil) || (c != nil && n != nil && *c == *n) {
			continue
		}
		var name string
		if err := s.dbPool.Pool.QueryRow(ctx, `SELECT name_ru FROM attribute_definitions WHERE id = $1`, id).Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, RevisionAttributeChange{Name: name, Current: c, Proposed: n})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}
