package products

import (
	"context"
	"fmt"
	"github.com/google/uuid"
)

func (s *Service) validateProductForModeration(ctx context.Context, p *Product) error {
	if p.CategoryID == nil {
		return fmt.Errorf("category is required for moderation")
	}

	// Fetch schema
	schemaRaw, err := s.GetCategoryAttributeSchema(ctx, *p.CategoryID)
	if err != nil {
		return err
	}
	schema := schemaRaw.(map[string]interface{})
	
	// Convert Product.Attributes to map
	pAttrMap := make(map[uuid.UUID]bool)
	for _, a := range p.Attributes {
		pAttrMap[a.AttributeDefinitionID] = true
	}

	// Validate required product attributes
	attrs := schema["attributes"].([]map[string]interface{})
	for _, a := range attrs {
		scope := a["scope"].(string)
		req := a["required"].(bool)
		defID := a["id"].(uuid.UUID)
		code := a["code"].(string)
		
		if scope == "PRODUCT" && req {
			if !pAttrMap[defID] {
				return fmt.Errorf("required product attribute missing: %s", code)
			}
		}
	}

	// Validate composition if required
	// ... (assuming no special requirement for now unless in schema)

	// Validate size chart
	chartReq := schema["sizeChartRequired"].(bool)
	if chartReq {
		if p.SizeChart == nil || len(p.SizeChart.Rows) == 0 {
			return fmt.Errorf("category requires a size chart")
		}
	}

	// Validate variants
	if len(p.Variants) == 0 {
		return fmt.Errorf("at least one variant is required")
	}

	for _, v := range p.Variants {
		// Prices
		if v.PriceCents == nil || *v.PriceCents <= 0 {
			return fmt.Errorf("variant price must be greater than 0")
		}
		
		// Variant attributes
		vAttrMap := make(map[uuid.UUID]bool)
		for _, a := range v.Attributes {
			vAttrMap[a.AttributeDefinitionID] = true
		}
		for _, a := range attrs {
			scope := a["scope"].(string)
			req := a["required"].(bool)
			defID := a["id"].(uuid.UUID)
			code := a["code"].(string)
			
			if scope == "VARIANT" && req {
				if !vAttrMap[defID] {
					return fmt.Errorf("required variant attribute missing: %s", code)
				}
			}
			if a["valueSource"].(string) == "VARIANT_COLOR" && req {
				if v.ColorID == nil {
					return fmt.Errorf("color is required for variant")
				}
			}
			if a["valueSource"].(string) == "VARIANT_SIZE" && req {
				if v.SizeValueID == nil {
					return fmt.Errorf("size is required for variant")
				}
			}
		}
	}

	return nil
}
