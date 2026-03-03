package engine

import (
	"fmt"
)

type NodeHandler interface {
	Execute(ctx *ExecutionContext, node Node) error
}

type Interceptor func(node Node, ctx *ExecutionContext, next func() error) error

type WorkflowEngine struct {
	handlers     map[string]NodeHandler
	interceptors []Interceptor
}

func NewWorkflowEngine() *WorkflowEngine {
	return &WorkflowEngine{
		handlers: make(map[string]NodeHandler),
	}
}

// Register mendaftarkan plugin/komponen eksekusi (LLM, RAG, Webhook)
func (e *WorkflowEngine) Register(nodeType string, handler NodeHandler) {
	e.handlers[nodeType] = handler
}

// Use menambahkan middleware/interceptor ke engine
func (e *WorkflowEngine) Use(i Interceptor) {
	e.interceptors = append(e.interceptors, i)
}

// Run mengeksekusi JSON workflow dari awal hingga akhir
func (e *WorkflowEngine) Run(graph *VisualGraph, initialData map[string]interface{}) (*ExecutionContext, error) {
	sortedNodes, err := TopologicalSort(graph)
	if err != nil {
		return nil, err // Tolak eksekusi jika ada infinite loop
	}

	ctx := NewExecutionContext()
	for k, v := range initialData {
		ctx.Set(k, v)
	}

	// Eksekusi berurutan (Bisa di-upgrade ke eksekusi paralel di masa depan)
	for _, node := range sortedNodes {
		err := e.executeNode(node, ctx)
		if err != nil {
			return nil, fmt.Errorf("kegagalan node [%s - %s]: %w", node.ID, node.Type, err)
		}
	}

	return ctx, nil
}

func (e *WorkflowEngine) executeNode(node Node, ctx *ExecutionContext) error {
	handler, exists := e.handlers[node.Type]
	if !exists {
		return fmt.Errorf("registry error: tidak ada handler untuk node tipe '%s'", node.Type)
	}

	// Build the interceptor chain
	// We want to execute interceptors in order: i[0] -> i[1] -> ... -> handler.Execute
	chain := func() error {
		return handler.Execute(ctx, node)
	}

	// Loop backwards to wrap the chain
	for i := len(e.interceptors) - 1; i >= 0; i-- {
		currentInterceptor := e.interceptors[i]
		nextFunc := chain
		chain = func() error {
			return currentInterceptor(node, ctx, nextFunc)
		}
	}

	return chain()
}
