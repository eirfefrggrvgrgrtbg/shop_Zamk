package search

type ResultType string

const (
	ResultTypeOrder          ResultType = "order"
	ResultTypeReturn         ResultType = "return"
	ResultTypeInventoryUnit  ResultType = "inventory_unit"
	ResultTypeProductVariant ResultType = "product_variant"
	ResultTypeProduct        ResultType = "product"
	ResultTypeCustomer       ResultType = "customer"
)

type GlobalSearchResult struct {
	Type                ResultType `json:"type"`
	ID                  string     `json:"id"`
	Title               string     `json:"title"`
	Subtitle            string     `json:"subtitle"`
	CanonicalIdentifier string     `json:"canonicalIdentifier"`
	NavigationTarget    string     `json:"navigationTarget"`
}

type GlobalSearchResponse struct {
	Results []GlobalSearchResult `json:"results"`
}

type AllowedPermissions struct {
	CanReadOrders    bool
	CanReadReturns   bool
	CanReadInventory bool
	CanReadProducts  bool
	CanReadUsers     bool
}

// HasAny returns true if at least one read permission is granted.
func (p AllowedPermissions) HasAny() bool {
	return p.CanReadOrders || p.CanReadReturns || p.CanReadInventory || p.CanReadProducts || p.CanReadUsers
}
