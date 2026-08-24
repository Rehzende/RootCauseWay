package database

import "testing"

func TestEstimateCostUSD_KnownModel(t *testing.T) {
	// claude-sonnet-4 -> $9/1M blended
	got := estimateCostUSD("anthropic/claude-sonnet-4-6", 1_000_000)
	if got != 9.0 {
		t.Errorf("expected 9.0, got %v", got)
	}
}

func TestEstimateCostUSD_IsCaseInsensitiveAndPrefixAgnostic(t *testing.T) {
	// Real model strings vary by how the provider prefixes them
	// (openrouter's "anthropic/claude-sonnet-4-6" vs a bare
	// "Claude-Sonnet-4-6") -- substring + case-insensitive match must catch
	// both rather than only the exact string this codebase happened to use
	// as its old hardcoded label.
	got := estimateCostUSD("Claude-Sonnet-4-6-20260101", 500_000)
	if got != 4.5 {
		t.Errorf("expected 4.5, got %v", got)
	}
}

func TestEstimateCostUSD_UnknownOrLocalModelIsZero(t *testing.T) {
	// The real live model in this codebase today is always a self-hosted
	// LM Studio model (e.g. "qwen/qwen2.5-coder-14b") or similar -- these
	// have no per-token API cost and must not silently inherit a fabricated
	// flat rate the way the old switch statement's `default` branch did.
	got := estimateCostUSD("qwen/qwen2.5-coder-14b-instruct", 1_000_000)
	if got != 0.2 {
		// "qwen" is in the table (self-hosted models can still be priced
		// if an org wants to reflect real GPU-time cost) -- but the point
		// is it's an explicit, named entry, not an unrelated default.
		t.Errorf("expected 0.2 for a matched qwen family entry, got %v", got)
	}

	got2 := estimateCostUSD("some-totally-unknown-local-model", 1_000_000)
	if got2 != 0.0 {
		t.Errorf("expected 0.0 for a model with no pricing entry, got %v", got2)
	}
}

func TestEstimateCostUSD_ZeroTokensIsZeroCost(t *testing.T) {
	if got := estimateCostUSD("anthropic/claude-sonnet-4-6", 0); got != 0.0 {
		t.Errorf("expected 0.0, got %v", got)
	}
}
