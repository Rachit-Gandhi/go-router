package pricing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
)

const anthropicPricingURL = "https://www.anthropic.com/pricing"

type Quote struct {
	Provider           string  `json:"provider"`
	Model              string  `json:"model"`
	InputPricePerMTok  float64 `json:"input_price_per_mtok"`
	OutputPricePerMTok float64 `json:"output_price_per_mtok"`
	Currency           string  `json:"currency"`
	Source             string  `json:"source"`
}

type SyncResult struct {
	Created int
	Updated int
	Skipped int
	Failed  int
}

type Fetcher interface {
	Name() string
	Fetch(ctx context.Context) ([]Quote, error)
}

type fetcherFunc struct {
	name string
	fn   func(ctx context.Context) ([]Quote, error)
}

func (f fetcherFunc) Name() string { return f.name }
func (f fetcherFunc) Fetch(ctx context.Context) ([]Quote, error) {
	return f.fn(ctx)
}

// DefaultFetchers returns built-in official source fetchers and curated fallbacks.
func DefaultFetchers() []Fetcher {
	return []Fetcher{
		fetcherFunc{name: "anthropic-pricing-page", fn: fetchAnthropicPricing},
		fetcherFunc{name: "static-price-catalog", fn: fetchStaticCatalog},
	}
}

func SyncQuotes(ctx context.Context, queries *dbquery.Queries, now time.Time, fetchers []Fetcher) (SyncResult, error) {
	if queries == nil {
		return SyncResult{}, errors.New("queries is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if len(fetchers) == 0 {
		fetchers = DefaultFetchers()
	}

	merged := make(map[string]Quote)
	result := SyncResult{}
	for _, fetcher := range fetchers {
		quotes, err := fetcher.Fetch(ctx)
		if err != nil {
			result.Failed++
			continue
		}
		for _, q := range quotes {
			q.Provider = strings.ToLower(strings.TrimSpace(q.Provider))
			q.Model = strings.TrimSpace(q.Model)
			q.Currency = strings.ToUpper(strings.TrimSpace(q.Currency))
			q.Source = strings.TrimSpace(q.Source)
			if q.Provider == "" || q.Model == "" || q.Currency == "" || q.Source == "" {
				continue
			}
			if q.InputPricePerMTok < 0 || q.OutputPricePerMTok < 0 {
				continue
			}
			merged[q.Provider+":"+q.Model] = q
		}
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		quote := merged[key]
		active, err := queries.GetActiveModelPricingByProviderModel(ctx, dbquery.GetActiveModelPricingByProviderModelParams{
			Provider: quote.Provider,
			Model:    quote.Model,
		})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return result, fmt.Errorf("load active model pricing for %s/%s: %w", quote.Provider, quote.Model, err)
		}

		if err == nil && pricingMatches(active, quote) {
			result.Skipped++
			continue
		}

		if err == nil {
			if _, closeErr := queries.CloseActiveModelPricing(ctx, dbquery.CloseActiveModelPricingParams{
				Provider:    quote.Provider,
				Model:       quote.Model,
				EffectiveTo: sql.NullTime{Time: now, Valid: true},
			}); closeErr != nil {
				return result, fmt.Errorf("close active model pricing for %s/%s: %w", quote.Provider, quote.Model, closeErr)
			}
			result.Updated++
		} else {
			result.Created++
		}

		if _, createErr := queries.CreateModelPricing(ctx, dbquery.CreateModelPricingParams{
			Provider:           quote.Provider,
			Model:              quote.Model,
			InputPricePerMtok:  quote.InputPricePerMTok,
			OutputPricePerMtok: quote.OutputPricePerMTok,
			Currency:           quote.Currency,
			Source:             quote.Source,
			EffectiveFrom:      now,
		}); createErr != nil {
			return result, fmt.Errorf("create model pricing for %s/%s: %w", quote.Provider, quote.Model, createErr)
		}
	}

	return result, nil
}

func pricingMatches(active dbquery.ModelPricing, quote Quote) bool {
	return strings.EqualFold(active.Provider, quote.Provider) &&
		active.Model == quote.Model &&
		active.InputPricePerMtok == quote.InputPricePerMTok &&
		active.OutputPricePerMtok == quote.OutputPricePerMTok &&
		strings.EqualFold(active.Currency, quote.Currency) &&
		active.Source == quote.Source
}

func fetchAnthropicPricing(ctx context.Context) ([]Quote, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, anthropicPricingURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic pricing request returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseAnthropicPricingHTML(string(body))
}

var (
	anthropicCardPattern  = regexp.MustCompile(`(?s)<h3[^>]*class="[^"]*card_pricing_title_text[^"]*"[^>]*>([^<]+)</h3>.*?tokens_main_label[^>]*>\s*Input\s*</div>.*?data-value="([0-9]+(?:\.[0-9]+)?)".*?tokens_main_label[^>]*>\s*Output\s*</div>.*?data-value="([0-9]+(?:\.[0-9]+)?)"`)
	normalizeModelPattern = regexp.MustCompile(`[^a-z0-9]+`)
)

func parseAnthropicPricingHTML(html string) ([]Quote, error) {
	matches := anthropicCardPattern.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil, errors.New("no anthropic model cards found in pricing HTML")
	}

	seen := make(map[string]struct{})
	quotes := make([]Quote, 0, len(matches)*2)
	for _, m := range matches {
		displayName := strings.TrimSpace(m[1])
		model := normalizeModelName(displayName)
		if model == "" {
			continue
		}
		inputPrice, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		outputPrice, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			continue
		}

		for _, provider := range []string{"anthropic", "claude"} {
			key := provider + ":" + model
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			quotes = append(quotes, Quote{
				Provider:           provider,
				Model:              model,
				InputPricePerMTok:  inputPrice,
				OutputPricePerMTok: outputPrice,
				Currency:           "USD",
				Source:             anthropicPricingURL,
			})
		}
	}
	if len(quotes) == 0 {
		return nil, errors.New("anthropic pricing parse produced no rows")
	}
	return quotes, nil
}

func normalizeModelName(displayName string) string {
	s := strings.ToLower(strings.TrimSpace(displayName))
	s = normalizeModelPattern.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func fetchStaticCatalog(context.Context) ([]Quote, error) {
	blob, err := staticCatalogFS.ReadFile("data/static_prices.json")
	if err != nil {
		return nil, err
	}
	var quotes []Quote
	if err := json.Unmarshal(blob, &quotes); err != nil {
		return nil, err
	}
	for i := range quotes {
		quotes[i].Provider = strings.ToLower(strings.TrimSpace(quotes[i].Provider))
		quotes[i].Model = strings.TrimSpace(quotes[i].Model)
		quotes[i].Currency = strings.ToUpper(strings.TrimSpace(quotes[i].Currency))
		quotes[i].Source = strings.TrimSpace(quotes[i].Source)
	}
	return quotes, nil
}
