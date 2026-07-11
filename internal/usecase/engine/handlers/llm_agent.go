package handlers

import (
	"fmt"
	"log"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/telemetry"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/engine"
)

type LLMAgentHandler struct {
	// Masukkan AI Client (OpenAI/Gemini) dari layer infrastruktur di sini
}

func NewLLMAgentHandler() *LLMAgentHandler {
	return &LLMAgentHandler{}
}

func (h *LLMAgentHandler) Execute(ctx *engine.ExecutionContext, node engine.Node) error {
	// 1. Ekstrak tenant_id untuk metrics
	tenantIDStr, ok := ctx.Get("tenant_id")
	tenantID := "unknown"
	if ok {
		if s, ok := tenantIDStr.(string); ok && s != "" {
			tenantID = s
		}
	}

	// 2. Ekstrak konfigurasi dari JSON frontend
	promptTemplate, ok := node.Data["prompt"].(string)
	if !ok {
		return fmt.Errorf("node %s: kehilangan konfigurasi 'prompt'", node.ID)
	}

	// 2. Baca state memori (misal input dari node sebelumnya)
	inputData, _ := ctx.Get("global_input")

	// 3. Logika komputasi nyata (Panggil API LLM)
	log.Printf("Mengeksekusi LLM Agent [Node: %s] dengan input: %v", node.ID, inputData)

	// TODO: Panggil infra AI Client Anda (e.g., openai.Complete(promptTemplate))
	mockResult := fmt.Sprintf("Simulasi respons AI untuk prompt: %s\n\nKonteks: %v", promptTemplate, inputData)

	// Simulate returning UsageMetadata from Gemini/OpenAI
	telemetry.TokenConsumption.WithLabelValues(tenantID, "mock-llm-model", "prompt").Add(float64(len(promptTemplate) / 4))
	telemetry.TokenConsumption.WithLabelValues(tenantID, "mock-llm-model", "completion").Add(50.0)

	// 5. Simpan hasil kembali ke Context agar bisa dibaca node selanjutnya
	ctx.Set(fmt.Sprintf("%s_result", node.ID), mockResult)

	return nil
}
