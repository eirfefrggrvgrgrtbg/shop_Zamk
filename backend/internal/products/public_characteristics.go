package products

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// PublicCharacteristic is one approved product characteristic as the buyer reads it (no ids, no technical codes).
type PublicCharacteristic struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// publicCharacteristics resolves the LIVE (approved) product-scope characteristics of a published product into
// readable name/value pairs in the category's order. Composition and size chart have their own blocks and are
// excluded, as are variant axes (colour, size), inactive definitions and empty values. Pending revision data is
// never read here: it lives only in product_revisions until approval writes product_attribute_values.
func (s *Service) publicCharacteristics(ctx context.Context, p *Product) ([]PublicCharacteristic, error) {
	if len(p.Attributes) == 0 {
		return nil, nil
	}
	type def struct {
		name  string
		order int
	}
	defs := map[uuid.UUID]def{}
	var ids []uuid.UUID
	for _, a := range p.Attributes {
		ids = append(ids, a.AttributeDefinitionID)
	}
	rows, err := s.dbPool.Pool.Query(ctx, `
		SELECT ad.id, ad.name_ru, COALESCE(cad.sort_order, 1000000)
		FROM attribute_definitions ad
		LEFT JOIN category_attribute_definitions cad ON cad.attribute_definition_id = ad.id AND cad.category_id = $2
		WHERE ad.id = ANY($1) AND ad.is_active AND ad.scope = 'PRODUCT'
		  AND COALESCE(ad.value_source, '') NOT IN ('MATERIAL_COMPOSITION', 'VARIANT_COLOR', 'VARIANT_SIZE')`, ids, p.CategoryID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uuid.UUID
		var d def
		if err := rows.Scan(&id, &d.name, &d.order); err != nil {
			rows.Close()
			return nil, err
		}
		defs[id] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	values := map[uuid.UUID][]string{}
	var order []uuid.UUID
	for _, a := range p.Attributes {
		if _, ok := defs[a.AttributeDefinitionID]; !ok {
			continue
		}
		var v string
		switch {
		case a.EnumValueID != nil:
			if err := s.dbPool.Pool.QueryRow(ctx, `SELECT name_ru FROM attribute_dictionary_values WHERE id = $1 AND is_active`, *a.EnumValueID).Scan(&v); err != nil {
				continue // an unknown or retired dictionary value is not shown rather than shown as an id
			}
		case a.TextValue != nil:
			v = *a.TextValue
		case a.NumberValue != nil:
			v = strconv.FormatFloat(*a.NumberValue, 'f', -1, 64)
		case a.BoolValue != nil:
			if *a.BoolValue {
				v = "Да"
			} else {
				v = "Нет"
			}
		}
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		if _, seen := values[a.AttributeDefinitionID]; !seen {
			order = append(order, a.AttributeDefinitionID)
		}
		values[a.AttributeDefinitionID] = append(values[a.AttributeDefinitionID], v)
	}
	// category order, then name
	for i := 1; i < len(order); i++ {
		for j := i; j > 0; j-- {
			a, b := defs[order[j-1]], defs[order[j]]
			if a.order < b.order || (a.order == b.order && a.name <= b.name) {
				break
			}
			order[j-1], order[j] = order[j], order[j-1]
		}
	}
	out := make([]PublicCharacteristic, 0, len(order))
	for _, id := range order {
		out = append(out, PublicCharacteristic{Name: defs[id].name, Value: strings.Join(values[id], ", ")})
	}
	return out, nil
}
