package pricing

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeModelName(t *testing.T) {
	tests := map[string]string{
		"Opus 4.5":     "opus-4-5",
		" Sonnet 4 ":   "sonnet-4",
		"Haiku_3":      "haiku-3",
		"Claude-3.7!":  "claude-3-7",
		"gpt-4o-mini ": "gpt-4o-mini",
	}
	for input, expected := range tests {
		if got := normalizeModelName(input); got != expected {
			t.Fatalf("normalizeModelName(%q): expected %q, got %q", input, expected, got)
		}
	}
}

func TestParseAnthropicPricingHTML(t *testing.T) {
	html := `
<div class="card_pricing_api_wrap">
	<h3 class="card_pricing_title_text">Sonnet 4</h3>
	<div class="tokens_main_label">Input</div>
	<div data-api-price=""><span data-value="3" class="tokens_main_val_number">3</span></div>
	<div class="tokens_main_label">Output</div>
	<div data-api-price=""><span data-value="15" class="tokens_main_val_number">15</span></div>
</div>
<div class="card_pricing_api_wrap">
	<h3 class="card_pricing_title_text">Haiku 3</h3>
	<div class="tokens_main_label">Input</div>
	<div data-api-price=""><span data-value="0.25" class="tokens_main_val_number">0.25</span></div>
	<div class="tokens_main_label">Output</div>
	<div data-api-price=""><span data-value="1.25" class="tokens_main_val_number">1.25</span></div>
</div>
`
	quotes, err := parseAnthropicPricingHTML(html)
	if err != nil {
		t.Fatalf("parseAnthropicPricingHTML: %v", err)
	}
	if len(quotes) != 4 {
		t.Fatalf("expected 4 quotes (anthropic+claude x 2 models), got %d", len(quotes))
	}

	seen := map[string]Quote{}
	for _, q := range quotes {
		seen[q.Provider+":"+q.Model] = q
	}
	for _, key := range []string{"anthropic:sonnet-4", "claude:sonnet-4", "anthropic:haiku-3", "claude:haiku-3"} {
		if _, ok := seen[key]; !ok {
			t.Fatalf("missing expected quote %q in %#v", key, seen)
		}
	}
	if seen["anthropic:sonnet-4"].InputPricePerMTok != 3 || seen["anthropic:sonnet-4"].OutputPricePerMTok != 15 {
		t.Fatalf("unexpected sonnet prices: %#v", seen["anthropic:sonnet-4"])
	}
}

func TestFetchStaticCatalog(t *testing.T) {
	quotes, err := fetchStaticCatalog(context.Background())
	if err != nil {
		t.Fatalf("fetchStaticCatalog: %v", err)
	}
	if len(quotes) < 2 {
		t.Fatalf("expected at least 2 static quotes, got %d", len(quotes))
	}

	var sawOpenAIMini bool
	for _, q := range quotes {
		if q.Provider == "openai" && q.Model == "gpt-4o-mini" {
			sawOpenAIMini = true
			if q.InputPricePerMTok <= 0 || q.OutputPricePerMTok <= 0 {
				t.Fatalf("expected positive openai gpt-4o-mini prices, got %#v", q)
			}
			if strings.ToUpper(q.Currency) != "USD" {
				t.Fatalf("expected USD currency, got %#v", q.Currency)
			}
		}
	}
	if !sawOpenAIMini {
		t.Fatalf("expected static catalog to include openai:gpt-4o-mini")
	}
}
