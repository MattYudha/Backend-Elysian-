package parsing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DocumentParser routes documents to the appropriate parsing backend:
//   - .txt / .md  → local plain-text extractor (no external dependency)
//   - .pdf / .docx / .pptx → Docling API (enterprise-grade layout + OCR)
//     Fallback: Unstructured.io if Docling is unavailable.
type DocumentParser struct {
	doclingURL      string
	unstructuredURL string
	httpClient      *http.Client
}

func NewDocumentParser(doclingURL, unstructuredURL string) *DocumentParser {
	return &DocumentParser{
		doclingURL:      doclingURL,
		unstructuredURL: unstructuredURL,
		httpClient:      &http.Client{Timeout: 120 * time.Second},
	}
}

// ExtractText dispatches to the correct parser based on file extension.
func (p *DocumentParser) ExtractText(ctx context.Context, filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".txt", ".md":
		return extractPlainText(filePath)
	case ".pdf", ".docx", ".pptx", ".xlsx":
		// Route to Docling for layout-aware extraction
		text, err := p.parseWithDocling(ctx, filePath)
		if err != nil {
			// Fallback to Unstructured.io if Docling unavailable
			return p.parseWithUnstructured(ctx, filePath)
		}
		return text, nil
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

// parseWithDocling sends the file to the Docling HTTP API and returns extracted Markdown text.
// Docling preserves table structure, multi-column layout, and uses OCR for scanned pages.
func (p *DocumentParser) parseWithDocling(ctx context.Context, filePath string) (string, error) {
	if p.doclingURL == "" {
		return "", fmt.Errorf("Docling URL not configured")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for Docling: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.doclingURL+"/v1alpha/convert/file", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Docling request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Docling returned HTTP %d", resp.StatusCode)
	}

	// Docling returns a JSON structure with markdown text per document
	var result struct {
		Document struct {
			MdContent string `json:"md_content"`
		} `json:"document"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Docling response: %w", err)
	}

	if result.Document.MdContent == "" {
		return "", fmt.Errorf("Docling returned empty content")
	}
	return result.Document.MdContent, nil
}

// parseWithUnstructured sends the file to Unstructured.io API (fallback).
func (p *DocumentParser) parseWithUnstructured(ctx context.Context, filePath string) (string, error) {
	if p.unstructuredURL == "" {
		return "", fmt.Errorf("Unstructured URL not configured")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for Unstructured: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.unstructuredURL+"/general/v0/general", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Unstructured request failed: %w", err)
	}
	defer resp.Body.Close()

	var elements []struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&elements); err != nil {
		return "", fmt.Errorf("failed to decode Unstructured response: %w", err)
	}

	var sb strings.Builder
	for _, el := range elements {
		sb.WriteString(el.Text)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String()), nil
}

// extractPlainText reads plain text / markdown files directly.
func extractPlainText(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
