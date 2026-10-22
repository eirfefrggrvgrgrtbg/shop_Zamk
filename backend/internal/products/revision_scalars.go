package products

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// revisionScalars are the products-row fields a pending revision proposes. A nil field is left unchanged on approval.
// It is the single reading of the snapshot shared by approval (PromoteProductRevision), the seller's Studio view and
// the admin diff, so what the seller edits, what Admin reviews and what approval applies cannot drift apart.
type revisionScalars struct {
	Title, Description, Gender, Color, Material, CareInstructions *string
	CategoryID, BrandID                                           *uuid.UUID
	PriceCents, OldPriceCents                                     *int64
	MainImageURL, MainImageObjectKey                              *string
}

func parseRevisionScalars(snapBytes []byte) revisionScalars {
	var f revisionScalars
	var pSnap Product
	var rawMap map[string]interface{}
	if len(snapBytes) > 0 {
		_ = json.Unmarshal(snapBytes, &pSnap)
		_ = json.Unmarshal(snapBytes, &rawMap)
	}
	str := func(typed *string, key string) *string {
		if typed != nil {
			return typed
		}
		if v, ok := rawMap[key].(string); ok {
			return &v
		}
		return nil
	}
	if pSnap.Title != "" {
		f.Title = &pSnap.Title
	} else if t, ok := rawMap["title"].(string); ok && t != "" {
		f.Title = &t
	}
	f.Description = str(pSnap.Description, "description")
	f.CategoryID = pSnap.CategoryID
	f.BrandID = pSnap.BrandID
	f.Gender = str(pSnap.Gender, "gender")
	f.Color = str(pSnap.Color, "color")
	f.Material = str(pSnap.Material, "material")
	f.CareInstructions = str(pSnap.CareInstructions, "careInstructions")
	if pSnap.PriceCents > 0 {
		f.PriceCents = &pSnap.PriceCents
	} else if pc, ok := rawMap["priceCents"].(float64); ok && pc > 0 {
		i := int64(pc)
		f.PriceCents = &i
	}
	if pSnap.OldPriceCents != nil {
		f.OldPriceCents = pSnap.OldPriceCents
	} else if opc, ok := rawMap["oldPriceCents"].(float64); ok {
		i := int64(opc)
		f.OldPriceCents = &i
	}
	f.MainImageURL = str(pSnap.MainImageURL, "mainImageUrl")
	f.MainImageObjectKey = str(pSnap.MainImageObjectKey, "mainImageObjectKey")
	return f
}

// RevisionFieldChange is one products-row field the pending revision changes (Admin review).
type RevisionFieldChange struct {
	Field    string  `json:"field"`
	Current  *string `json:"current,omitempty"`
	Proposed *string `json:"proposed,omitempty"`
}

// overlayRevisionScalars shows the seller the proposed values (Studio edits the target state, never the live one).
func overlayRevisionScalars(p *Product, f revisionScalars) {
	if f.Title != nil {
		p.Title = *f.Title
	}
	if f.Description != nil {
		p.Description = f.Description
	}
	if f.CategoryID != nil {
		p.CategoryID = f.CategoryID
	}
	if f.BrandID != nil {
		p.BrandID = f.BrandID
	}
	if f.Gender != nil {
		p.Gender = f.Gender
	}
	if f.Color != nil {
		p.Color = f.Color
	}
	if f.Material != nil {
		p.Material = f.Material
	}
	if f.CareInstructions != nil {
		p.CareInstructions = f.CareInstructions
	}
	if f.PriceCents != nil {
		p.PriceCents = *f.PriceCents
	}
	if f.OldPriceCents != nil {
		p.OldPriceCents = f.OldPriceCents
	}
}

// revisionFieldChanges lists the products-row fields whose proposed value differs from the live one.
func revisionFieldChanges(live *Product, f revisionScalars) []RevisionFieldChange {
	var out []RevisionFieldChange
	addStr := func(field string, cur, prop *string) {
		if prop == nil {
			return
		}
		if cur != nil && *cur == *prop {
			return
		}
		if cur == nil && *prop == "" {
			return
		}
		out = append(out, RevisionFieldChange{Field: field, Current: cur, Proposed: prop})
	}
	money := func(v *int64) *string {
		if v == nil {
			return nil
		}
		s := fmt.Sprintf("%d", *v)
		return &s
	}
	id := func(v *uuid.UUID) *string {
		if v == nil {
			return nil
		}
		s := v.String()
		return &s
	}
	addStr("title", &live.Title, f.Title)
	addStr("description", live.Description, f.Description)
	addStr("categoryId", id(live.CategoryID), id(f.CategoryID))
	addStr("brandId", id(live.BrandID), id(f.BrandID))
	addStr("gender", live.Gender, f.Gender)
	addStr("color", live.Color, f.Color)
	addStr("material", live.Material, f.Material)
	addStr("careInstructions", live.CareInstructions, f.CareInstructions)
	addStr("priceCents", money(&live.PriceCents), money(f.PriceCents))
	addStr("oldPriceCents", money(live.OldPriceCents), money(f.OldPriceCents))
	return out
}
