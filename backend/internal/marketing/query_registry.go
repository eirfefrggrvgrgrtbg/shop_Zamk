package marketing

type DimensionDef struct {
	Key             string
	Kind            string // "dimension"
	Type            string // "string", "date"
	SafeColumns     []QueryColumn
	SessionsExpr    string
	OrdersExpr      string
	BehavioralExpr  string
	ReturnsExpr     string
	EntityJoinSQL   string
	EntitySelectSQL string
}

type MetricDef struct {
	Key             string
	Kind            string // "metric"
	Type            string // "number", "money", "percentage"
	SafeColumn      QueryColumn
	IsAvailableWith func(dimensions []string) bool
}

func GetRegistryDimensions() []DimensionDef {
	return []DimensionDef{
		{
			Key:  "day",
			Kind: "dimension",
			Type: "date",
			SafeColumns: []QueryColumn{
				{Key: "day", Kind: "dimension", Type: "date"},
			},
			SessionsExpr:    "COALESCE(DATE_TRUNC('day', s.started_at)::date::text, '')",
			OrdersExpr:      "COALESCE(DATE_TRUNC('day', dpo.paid_at)::date::text, '')",
			BehavioralExpr:  "COALESCE(DATE_TRUNC('day', b.occurred_at)::date::text, '')",
			ReturnsExpr:     "COALESCE(DATE_TRUNC('day', COALESCE(r.completed_at, r.updated_at))::date::text, '')",
			EntityJoinSQL:   "",
			EntitySelectSQL: "a.dim_%d AS day",
		},
		{
			Key:  "source",
			Kind: "dimension",
			Type: "string",
			SafeColumns: []QueryColumn{
				{Key: "sourceKey", Kind: "dimension", Type: "string"},
				{Key: "sourceKind", Kind: "dimension", Type: "string"},
				{Key: "source", Kind: "dimension", Type: "string"},
			},
			SessionsExpr:    "COALESCE(s.source, '')",
			OrdersExpr:      "COALESCE(dpo.source, '')",
			BehavioralExpr:  "COALESCE(b.source, '')",
			ReturnsExpr:     "COALESCE(oa.source, '')",
			EntityJoinSQL:   "",
			EntitySelectSQL: "a.dim_%d AS _raw_source_%d",
		},
		{
			Key:  "campaign",
			Kind: "dimension",
			Type: "string",
			SafeColumns: []QueryColumn{
				{Key: "campaignName", Kind: "dimension", Type: "string"},
			},
			SessionsExpr:    "COALESCE(s.campaign_id::text, '')",
			OrdersExpr:      "COALESCE(dpo.campaign_id::text, '')",
			BehavioralExpr:  "''",
			ReturnsExpr:     "COALESCE(oa.campaign_id::text, '')",
			EntityJoinSQL:   "LEFT JOIN marketing_campaigns mc%d ON mc%d.id = NULLIF(a.dim_%d, '')::uuid",
			EntitySelectSQL: "COALESCE(mc%d.title, 'Без кампании') AS campaign_name",
		},
		{
			Key:  "product",
			Kind: "dimension",
			Type: "string",
			SafeColumns: []QueryColumn{
				{Key: "productName", Kind: "dimension", Type: "string"},
			},
			SessionsExpr:    "''",
			OrdersExpr:      "COALESCE(oi.product_id::text, '')",
			BehavioralExpr:  "COALESCE(b.product_id::text, '')",
			ReturnsExpr:     "COALESCE(oi.product_id::text, '')",
			EntityJoinSQL:   "LEFT JOIN products pr%d ON pr%d.id = NULLIF(a.dim_%d, '')::uuid",
			EntitySelectSQL: "pr%d.title AS product_name",
		},
		{
			Key:  "designer",
			Kind: "dimension",
			Type: "string",
			SafeColumns: []QueryColumn{
				{Key: "designerName", Kind: "dimension", Type: "string"},
			},
			SessionsExpr:    "''",
			OrdersExpr:      "COALESCE(pr_ord.seller_id::text, '')",
			BehavioralExpr:  "COALESCE(pr_beh.seller_id::text, '')",
			ReturnsExpr:     "COALESCE(pr_ret.seller_id::text, '')",
			EntityJoinSQL:   "LEFT JOIN sellers s%d ON s%d.id = NULLIF(a.dim_%d, '')::uuid",
			EntitySelectSQL: "s%d.brand_name AS designer_name",
		},
		{
			Key:  "category",
			Kind: "dimension",
			Type: "string",
			SafeColumns: []QueryColumn{
				{Key: "categoryName", Kind: "dimension", Type: "string"},
			},
			SessionsExpr:    "''",
			OrdersExpr:      "COALESCE(pr_ord.category_id::text, '')",
			BehavioralExpr:  "COALESCE(pr_beh.category_id::text, '')",
			ReturnsExpr:     "COALESCE(pr_ret.category_id::text, '')",
			EntityJoinSQL:   "LEFT JOIN categories cat%d ON cat%d.id = NULLIF(a.dim_%d, '')::uuid",
			EntitySelectSQL: "cat%d.name AS category_name",
		},
	}
}

func GetRegistryMetrics() []MetricDef {
	hasProductOrDesignerOrCategory := func(dims []string) bool {
		for _, d := range dims {
			if d == "product" || d == "designer" || d == "category" {
				return true
			}
		}
		return false
	}

	hasSourceOrCampaign := func(dims []string) bool {
		for _, d := range dims {
			if d == "source" || d == "campaign" {
				return true
			}
		}
		return false
	}

	return []MetricDef{
		{
			Key:        "sessions",
			Kind:       "metric",
			Type:       "number",
			SafeColumn: QueryColumn{Key: "sessions", Kind: "metric", Type: "number"},
			IsAvailableWith: func(dims []string) bool {
				// Sessions cannot be broken down by product, designer, or category in V1
				return !hasProductOrDesignerOrCategory(dims)
			},
		},
		{
			Key:        "orders",
			Kind:       "metric",
			Type:       "number",
			SafeColumn: QueryColumn{Key: "orders", Kind: "metric", Type: "number"},
			IsAvailableWith: func(dims []string) bool {
				return true
			},
		},
		{
			Key:        "sold_units",
			Kind:       "metric",
			Type:       "number",
			SafeColumn: QueryColumn{Key: "sold_units", Kind: "metric", Type: "number"},
			IsAvailableWith: func(dims []string) bool {
				return true
			},
		},
		{
			Key:        "revenue",
			Kind:       "metric",
			Type:       "money",
			SafeColumn: QueryColumn{Key: "revenue", Kind: "metric", Type: "money"},
			IsAvailableWith: func(dims []string) bool {
				return true
			},
		},
		{
			Key:        "product_views",
			Kind:       "metric",
			Type:       "number",
			SafeColumn: QueryColumn{Key: "product_views", Kind: "metric", Type: "number"},
			IsAvailableWith: func(dims []string) bool {
				// Behavioral events cannot be broken down by source or campaign in V1
				return !hasSourceOrCampaign(dims)
			},
		},
		{
			Key:        "favorites",
			Kind:       "metric",
			Type:       "number",
			SafeColumn: QueryColumn{Key: "favorites", Kind: "metric", Type: "number"},
			IsAvailableWith: func(dims []string) bool {
				return !hasSourceOrCampaign(dims)
			},
		},
		{
			Key:        "add_to_cart",
			Kind:       "metric",
			Type:       "number",
			SafeColumn: QueryColumn{Key: "add_to_cart", Kind: "metric", Type: "number"},
			IsAvailableWith: func(dims []string) bool {
				return !hasSourceOrCampaign(dims)
			},
		},
		{
			Key:        "conversion",
			Kind:       "metric",
			Type:       "percentage",
			SafeColumn: QueryColumn{Key: "conversion", Kind: "metric", Type: "percentage"},
			IsAvailableWith: func(dims []string) bool {
				// If both source/campaign AND product/designer/category are present, conversion is unsupported
				if hasSourceOrCampaign(dims) && hasProductOrDesignerOrCategory(dims) {
					return false
				}
				return true
			},
		},
		{
			Key:        "returns",
			Kind:       "metric",
			Type:       "number",
			SafeColumn: QueryColumn{Key: "returns", Kind: "metric", Type: "number"},
			IsAvailableWith: func(dims []string) bool {
				return true
			},
		},
		{
			Key:        "returned_units",
			Kind:       "metric",
			Type:       "number",
			SafeColumn: QueryColumn{Key: "returned_units", Kind: "metric", Type: "number"},
			IsAvailableWith: func(dims []string) bool {
				return true
			},
		},
	}
}

func findDimension(dims []DimensionDef, key string) *DimensionDef {
	for _, d := range dims {
		if d.Key == key {
			return &d
		}
	}
	return nil
}

func findMetric(metrics []MetricDef, key string) *MetricDef {
	for _, m := range metrics {
		if m.Key == key {
			return &m
		}
	}
	return nil
}
