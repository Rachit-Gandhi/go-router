package pricing

import "embed"

//go:embed data/static_prices.json
var staticCatalogFS embed.FS
