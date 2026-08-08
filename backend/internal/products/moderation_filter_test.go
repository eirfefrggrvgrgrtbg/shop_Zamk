package products

import (
	"strings"
	"testing"
)

func TestModerationFilter_ProblemFlagsLogic(t *testing.T) {
	tests := []struct {
		name          string
		filter        AdminProductFilter
		expectedFlags []string
	}{
		{
			name: "no_main_image flag set",
			filter: AdminProductFilter{
				NoMainImage: boolPtr(true),
			},
			expectedFlags: []string{"no_main_image"},
		},
		{
			name: "no_description flag set",
			filter: AdminProductFilter{
				NoDescription: boolPtr(true),
			},
			expectedFlags: []string{"no_description"},
		},
		{
			name: "no_brand flag set",
			filter: AdminProductFilter{
				NoBrand: boolPtr(true),
			},
			expectedFlags: []string{"no_brand"},
		},
		{
			name: "no_variants flag set",
			filter: AdminProductFilter{
				NoVariants: boolPtr(true),
			},
			expectedFlags: []string{"no_variants"},
		},
		{
			name: "no_price flag set",
			filter: AdminProductFilter{
				NoPrice: boolPtr(true),
			},
			expectedFlags: []string{"no_price"},
		},
		{
			name: "duplicate_sku flag set",
			filter: AdminProductFilter{
				DuplicateSKU: boolPtr(true),
			},
			expectedFlags: []string{"duplicate_sku"},
		},
		{
			name: "no_stock flag set",
			filter: AdminProductFilter{
				NoStock: boolPtr(true),
			},
			expectedFlags: []string{"no_stock"},
		},
		{
			name: "resubmitted flag set",
			filter: AdminProductFilter{
				Resubmitted: boolPtr(true),
			},
			expectedFlags: []string{"resubmitted"},
		},
		{
			name: "combination no_brand + no_variants",
			filter: AdminProductFilter{
				NoBrand:    boolPtr(true),
				NoVariants: boolPtr(true),
			},
			expectedFlags: []string{"no_brand", "no_variants"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := activeProblemFlags(tt.filter)
			if len(flags) != len(tt.expectedFlags) {
				t.Errorf("expected %d active flags, got %d (%v)", len(tt.expectedFlags), len(flags), flags)
			}
			for _, expected := range tt.expectedFlags {
				found := false
				for _, f := range flags {
					if f == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected flag %s not found in active flags %v", expected, flags)
				}
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func activeProblemFlags(f AdminProductFilter) []string {
	var flags []string
	if f.NoMainImage != nil && *f.NoMainImage {
		flags = append(flags, "no_main_image")
	}
	if f.NoDescription != nil && *f.NoDescription {
		flags = append(flags, "no_description")
	}
	if f.NoBrand != nil && *f.NoBrand {
		flags = append(flags, "no_brand")
	}
	if f.NoVariants != nil && *f.NoVariants {
		flags = append(flags, "no_variants")
	}
	if f.NoPrice != nil && *f.NoPrice {
		flags = append(flags, "no_price")
	}
	if f.DuplicateSKU != nil && *f.DuplicateSKU {
		flags = append(flags, "duplicate_sku")
	}
	if f.NoStock != nil && *f.NoStock {
		flags = append(flags, "no_stock")
	}
	if f.Resubmitted != nil && *f.Resubmitted {
		flags = append(flags, "resubmitted")
	}
	return flags
}

func TestProblemFlagsWhitelist(t *testing.T) {
	validFlags := []string{
		"noMainImage", "no_main_image",
		"noDescription", "no_description",
		"noBrand", "no_brand",
		"noVariants", "no_variants",
		"noPrice", "no_price",
		"duplicateSku", "duplicate_sku",
		"noStock", "no_stock",
		"resubmitted",
	}

	for _, flag := range validFlags {
		if !isWhitelistedProblemFlag(flag) {
			t.Errorf("expected %s to be whitelisted", flag)
		}
	}

	invalidFlags := []string{"drop_tables", "1=1", "unknown_flag", "; DELETE FROM products;"}
	for _, flag := range invalidFlags {
		if isWhitelistedProblemFlag(flag) {
			t.Errorf("expected %s NOT to be whitelisted", flag)
		}
	}
}

func isWhitelistedProblemFlag(flag string) bool {
	switch strings.TrimSpace(flag) {
	case "noMainImage", "no_main_image",
		"noDescription", "no_description",
		"noBrand", "no_brand",
		"noVariants", "no_variants",
		"noPrice", "no_price",
		"duplicateSku", "duplicate_sku",
		"noStock", "no_stock",
		"resubmitted":
		return true
	default:
		return false
	}
}
