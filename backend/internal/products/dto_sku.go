package products

type GenerateSKUsRequest struct {
	Count int `json:"count" validate:"required,min=1,max=100"`
}

type GenerateSKUsResponse struct {
	SKUs []string `json:"skus"`
}
