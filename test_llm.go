package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/minimax"
)

func main() {
	apiKey := "sk-Liql8MO7x-BO3kND-1zxXA"
	client := minimax.NewClient(apiKey)

	messages := []minimax.ChatMessage{
		{Role: "user", Content: "Hello, reply with OK"},
	}

	fmt.Println("Testing GenerateContent with LiteLLM Nusatek Proxy...")
	resp, usage, err := client.GenerateContent(context.Background(), "You are a helpful assistant.", messages)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Status: 200 OK")
	fmt.Printf("Response: %s\n", resp)
	fmt.Printf("Usage: %+v\n", usage)
}
