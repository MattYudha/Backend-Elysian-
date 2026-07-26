package minimax

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	apiKey    string
	baseURL   string
	modelName string
	client    *http.Client
}

func NewClient(apiKey string) *Client {
	baseURL := os.Getenv("OPENCODE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://ai-litellm-app.dev.ciptadusa.com/v1"
	}
	modelName := os.Getenv("OPENCODE_MODEL_NAME")
	if modelName == "" {
		modelName = "deepseek-chat"
	}

	return &Client{
		apiKey:    apiKey,
		baseURL:   baseURL,
		modelName: modelName,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type BaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type EmbeddingData struct {
	Embedding []float32 `json:"embedding"`
}

type EmbeddingResponse struct {
	Data     []EmbeddingData `json:"data"`
	BaseResp BaseResp        `json:"base_resp"`
}

func (c *Client) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	resp, err := c.EmbedBatch(ctx, []string{query}, "query")
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return resp[0], nil
}

func (c *Client) EmbedBatch(ctx context.Context, texts []string, embedType string) ([][]float32, error) {
	// If Gemini key is provided, route directly to Gemini API
	if strings.HasPrefix(c.apiKey, "AIzaSy") {
		return c.embedBatchGemini(ctx, texts)
	}

	// Fallback/Mock check for local development & offline testing
	isMock := c.apiKey == "" || 
		strings.HasPrefix(c.apiKey, "mock_")
	
	if isMock {
		log.Printf("⚠️ Minimax API Warning: Using mock embeddings (API key is blank, mock, or Gemini key)")
		mockVectors := make([][]float32, len(texts))
		for i := range texts {
			mockVectors[i] = make([]float32, 1536) // embo-01 uses 1536 dimensions
			for j := 0; j < 1536; j++ {
				mockVectors[i][j] = 0.001 * float32(j % 10)
			}
		}
		return mockVectors, nil
	}

	if embedType == "" {
		embedType = "db"
	}
	reqBody := EmbeddingRequest{
		Model: "text-embedding-3-small", // Assuming LiteLLM standard embedding model
		Input: texts,
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embeddings", bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-litellm-api-key", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("litellm api returned status: %d", resp.StatusCode)
	}

	var embedResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, err
	}

	vectors := make([][]float32, len(embedResp.Data))
	for i, data := range embedResp.Data {
		vectors[i] = data.Embedding
	}

	return vectors, nil
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	Choices  []ChatChoice `json:"choices"`
	BaseResp BaseResp     `json:"base_resp"`
	Usage    Usage        `json:"usage"`
}

func (c *Client) GenerateContent(ctx context.Context, systemInstruction string, messages []ChatMessage) (string, Usage, error) {
	// If Gemini key is provided, route directly to Gemini API
	if strings.HasPrefix(c.apiKey, "AIzaSy") {
		return c.generateContentGemini(ctx, systemInstruction, messages)
	}

	// Fallback/Mock check for local development & offline testing
	isMock := c.apiKey == "" || 
		strings.HasPrefix(c.apiKey, "mock_")
	
	if isMock {
		log.Printf("⚠️ Minimax API Warning: Using mock chat generation (API key is blank, mock, or Gemini key)")
		
		// Find user query and any context
		userQuery := ""
		contextRef := ""
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				userQuery = messages[i].Content
				break
			}
		}

		// Look for "Knowledge base reference:" in userQuery
		importRefIndex := strings.Index(userQuery, "Knowledge base reference:")
		if importRefIndex != -1 {
			contextRef = userQuery[importRefIndex:]
			userQuery = strings.TrimSpace(userQuery[:importRefIndex])
		}

		// Default friendly response
		var responseText string
		if contextRef != "" {
			responseText = fmt.Sprintf("Halo! Saya adalah Elysian AI Assistant (Mode Mock Lokal).\n\nBerdasarkan berkas yang tersimpan di Knowledge Base Anda:\n%s\n\nApakah ada hal lain dari data anggaran tersebut yang ingin Anda tanyakan?", contextRef)
		} else {
			responseText = fmt.Sprintf("Halo! Saya adalah Elysian AI Assistant (Mode Mock Lokal).\n\nAnda menanyakan: \"%s\".\n\nKarena sistem berjalan dalam mode pengujian lokal offline (Mock), saya tidak dapat memproses jawaban LLM secara riil. Silakan unggah berkas anggaran di **Knowledge Base** dan pastikan statusnya `Ready` agar saya dapat menampilkan referensi datanya di sini.", userQuery)
		}

		mockUsage := Usage{
			PromptTokens:     len(userQuery) / 4,
			CompletionTokens: len(responseText) / 4,
			TotalTokens:      (len(userQuery) + len(responseText)) / 4,
		}
		return responseText, mockUsage, nil
	}

	var reqMessages []ChatMessage
	if systemInstruction != "" {
		reqMessages = append(reqMessages, ChatMessage{
			Role:    "system",
			Content: systemInstruction,
		})
	}
	reqMessages = append(reqMessages, messages...)

	reqBody := ChatRequest{
		Model:    c.modelName,
		Messages: reqMessages,
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(reqJSON))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", Usage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errStr := strings.TrimSpace(string(bodyBytes))
		if errStr == "" {
			errStr = fmt.Sprintf("status code %d", resp.StatusCode)
		}
		return "", Usage{}, fmt.Errorf("Opencode API returned error (%d): %s", resp.StatusCode, errStr)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", Usage{}, err
	}

	if chatResp.BaseResp.StatusCode != 0 {
		return "", Usage{}, fmt.Errorf("Opencode API error: %s (code %d)", chatResp.BaseResp.StatusMsg, chatResp.BaseResp.StatusCode)
	}

	if len(chatResp.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("no response text returned from Opencode")
	}

	content := chatResp.Choices[0].Message.Content
	re := regexp.MustCompile(`(?s)<think>.*?</think>`)
	content = re.ReplaceAllString(content, "")
	return strings.TrimSpace(content), chatResp.Usage, nil
}

// ─── Gemini Integration Helper Structs & Methods ─────────────────

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiEmbedRequest struct {
	Model   string        `json:"model"`
	Content GeminiContent `json:"content"`
}

type GeminiBatchEmbedRequest struct {
	Requests []GeminiEmbedRequest `json:"requests"`
}

type GeminiEmbeddingValue struct {
	Values []float32 `json:"values"`
}

type GeminiBatchEmbedResponse struct {
	Embeddings []GeminiEmbeddingValue `json:"embeddings"`
}

type GeminiChatContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiSystemInstruction struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiChatRequest struct {
	Contents          []GeminiChatContent      `json:"contents"`
	SystemInstruction *GeminiSystemInstruction `json:"systemInstruction,omitempty"`
}

type GeminiCandidate struct {
	Content      GeminiChatContent `json:"content"`
	FinishReason string            `json:"finishReason"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type GeminiChatResponse struct {
	Candidates    []GeminiCandidate   `json:"candidates"`
	UsageMetadata GeminiUsageMetadata `json:"usageMetadata"`
}

func (c *Client) embedBatchGemini(ctx context.Context, texts []string) ([][]float32, error) {
	var geminiReqs []GeminiEmbedRequest
	for _, text := range texts {
		geminiReqs = append(geminiReqs, GeminiEmbedRequest{
			Model: "models/text-embedding-004",
			Content: GeminiContent{
				Parts: []GeminiPart{
					{Text: text},
				},
			},
		})
	}

	reqBody := GeminiBatchEmbedRequest{Requests: geminiReqs}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:batchEmbedContents?key=%s", c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ Gemini Embedding API returned status: %d. Falling back to mock embeddings.", resp.StatusCode)
		mockVectors := make([][]float32, len(texts))
		for i := range texts {
			mockVectors[i] = make([]float32, 1536)
			for j := 0; j < 1536; j++ {
				mockVectors[i][j] = 0.001 * float32(j % 10)
			}
		}
		return mockVectors, nil
	}

	var embedResp GeminiBatchEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, err
	}

	vectors := make([][]float32, len(embedResp.Embeddings))
	for i, emb := range embedResp.Embeddings {
		vectors[i] = emb.Values
	}

	return vectors, nil
}

func (c *Client) generateContentGemini(ctx context.Context, systemInstruction string, messages []ChatMessage) (string, Usage, error) {
	var contents []GeminiChatContent
	for _, msg := range messages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		} else if role != "user" && role != "model" {
			role = "user"
		}
		contents = append(contents, GeminiChatContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Content}},
		})
	}

	reqBody := GeminiChatRequest{
		Contents: contents,
	}

	if systemInstruction != "" {
		reqBody.SystemInstruction = &GeminiSystemInstruction{
			Parts: []GeminiPart{{Text: systemInstruction}},
		}
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqJSON))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", Usage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", Usage{}, fmt.Errorf("gemini api returned status: %d", resp.StatusCode)
	}

	var chatResp GeminiChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", Usage{}, err
	}

	if len(chatResp.Candidates) == 0 || len(chatResp.Candidates[0].Content.Parts) == 0 {
		return "", Usage{}, fmt.Errorf("no response candidates returned from gemini")
	}

	responseText := chatResp.Candidates[0].Content.Parts[0].Text

	usage := Usage{
		PromptTokens:     chatResp.UsageMetadata.PromptTokenCount,
		CompletionTokens: chatResp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      chatResp.UsageMetadata.TotalTokenCount,
	}

	return responseText, usage, nil
}

