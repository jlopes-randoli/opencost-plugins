package networkplugin

type ProductPrice struct {
	Product Product `json:"product"`
	Terms   Terms   `json:"terms"`
}

type Product struct {
	Attributes Attributes `json:"attributes"`
}

type Attributes struct {
	UsageType string `json:"usagetype"`
}

type Terms struct {
	OnDemand map[string]OnDemandTerm `json:"OnDemand"`
}

type OnDemandTerm struct {
	PriceDimensions map[string]PriceDimension `json:"priceDimensions"`
}

type PriceDimension struct {
	BeginRange   string       `json:"beginRange"`
	EndRange     string       `json:"endRange"`
	Unit         string       `json:"unit"`
	Description  string       `json:"description"`
	PricePerUnit PricePerUnit `json:"pricePerUnit"`
}

type PricePerUnit struct {
	USD string `json:"USD"`
}

type BilledUsage struct {
	UsageQuantityGB float32
	BilledCost      float32
}
