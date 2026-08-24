package database

import "strings"

// llmPricePerMillionTokens maps a case-insensitive model-name substring to
// an approximate blended (input+output average) USD price per 1M tokens.
// Checked in order -- first match wins, so a specific prefix must come
// before a broader family it's also a substring of.
//
// This is the single canonical pricing table for the whole backend --
// GetCostByModel and GetCostByIncident both call estimateCostUSD instead of
// each hardcoding their own switch statement (which is how a platform audit
// found three independent, mutually-inconsistent pricing constants: this
// one, a dead dict in agent-service/app/orchestrator/orchestrator.py, and
// frontend/src/pages/AnalyticsPage.tsx's SONNET_COST_PER_TOKEN).
//
// Self-hosted/local models (LM Studio, Ollama, vLLM -- everything this
// platform's own agents actually call today) have no per-token API cost and
// are deliberately left unmatched, so estimateCostUSD returns 0 for them
// rather than a fabricated flat rate. A future LLM-provider settings UI
// (see AnalyticsPage's "Cost by Model" card) is the right place for an org
// to attach a real $/token figure to a self-hosted deployment if they want
// electricity/GPU-amortized cost reflected here.
var llmPricePerMillionTokens = []struct {
	substr string
	usd    float64
}{
	{"claude-3-5-sonnet", 9.0},
	{"claude-3-7-sonnet", 9.0},
	{"claude-sonnet-4", 9.0},
	{"claude-3-opus", 30.0},
	{"claude-3-5-haiku", 1.6},
	{"claude-3-haiku", 0.5},
	{"gpt-4o-mini", 0.375},
	{"gpt-4o", 5.0},
	{"gpt-4-turbo", 20.0},
	{"gpt-3.5-turbo", 1.0},
	{"llama-3.1-70b", 0.4},
	{"llama-3.1-8b", 0.06},
	{"llama-3", 0.2},
	{"mixtral", 0.5},
	{"qwen", 0.2},
	{"deepseek", 0.3},
	{"gemini-1.5-pro", 5.0},
	{"gemini-1.5-flash", 0.3},
}

// estimateCostUSD returns an approximate USD cost for `tokens` total tokens
// consumed by `model`, based on a blended input+output rate. Returns 0 for
// models not in the table (typically self-hosted/local models with no
// per-token API cost) rather than a fabricated flat rate.
func estimateCostUSD(model string, tokens int) float64 {
	m := strings.ToLower(model)
	for _, p := range llmPricePerMillionTokens {
		if strings.Contains(m, p.substr) {
			return float64(tokens) / 1_000_000 * p.usd
		}
	}
	return 0
}
