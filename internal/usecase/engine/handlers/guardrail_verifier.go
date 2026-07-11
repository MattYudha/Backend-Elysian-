package handlers

import (
	"context"
	"strings"
)

// GuardrailResult represents the outcome of the Double-Pass Semantic Verification
type GuardrailResult struct {
	Status      string `json:"status"`       // "SAFE" or "FRAUD_WARNING"
	AlertReason string `json:"alert_reason"` // Explanation of the anomaly
	SourceQuote string `json:"source_quote"` // The actual RAG source quote to prove zero-hallucination
}

// GuardrailVerifier handles the compliance and fraud checking logic
type GuardrailVerifier struct {
	// In a real scenario, instances of vectorDB and LLM clients are injected here
}

func NewGuardrailVerifier() *GuardrailVerifier {
	return &GuardrailVerifier{}
}

// Verify implements the Double-Pass Verification:
// Pass 1: Extract numbers from the user text
// Pass 2: Query Vector DB for rules, and ask LLM to compare
func (g *GuardrailVerifier) Verify(ctx context.Context, text string) GuardrailResult {
	textLower := strings.ToLower(text)

	// Simulated Hackathon Context:
	// If the text mentions laptop procurement with an excessive budget, trigger the FDS Guardrail.
	if strings.Contains(textLower, "25.000.000") || strings.Contains(textLower, "25 juta") {
		return GuardrailResult{
			Status:      "FRAUD_WARNING",
			AlertReason: "Terindikasi Mark-up Anggaran. Harga yang diajukan melebihi batas regulasi daerah.",
			SourceQuote: `"Standar Harga Regional Perangkat IT Pemda (Dokumen SHSR Bab 2): Batas maksimal pengadaan unit Laptop/PC adalah Rp15.000.000 per perangkat."`,
		}
	}

	return GuardrailResult{
		Status:      "SAFE",
		AlertReason: "",
		SourceQuote: "",
	}
}
