package interceptors

import (
	"regexp"
)

// PiiRedactor is responsible for detecting and masking sensitive data (PII)
type PiiRedactor struct {
	nikRegex  *regexp.Regexp
	npwpRegex *regexp.Regexp
}

// NewPiiRedactor initializes the compiled regular expressions for PII detection
func NewPiiRedactor() *PiiRedactor {
	return &PiiRedactor{
		// Matches exactly 16 digits (NIK) and captures the last 4 digits
		nikRegex: regexp.MustCompile(`\b(\d{12})(\d{4})\b`),
		
		// Matches exactly 15 digits (NPWP format without punctuation)
		npwpRegex: regexp.MustCompile(`\b(\d{11})(\d{4})\b`),
	}
}

// Redact parses the incoming text and masks NIK/NPWP before reaching the LLM or external APIs.
// E.g., "3171234567890123" becomes "************0123"
func (p *PiiRedactor) Redact(text string) string {
	// Mask NIK: replace first 12 digits with '*', keep last 4
	redactedText := p.nikRegex.ReplaceAllString(text, "************$2")
	
	// Mask NPWP: replace first 11 digits with '*', keep last 4
	redactedText = p.npwpRegex.ReplaceAllString(redactedText, "***********$2")
	
	return redactedText
}
