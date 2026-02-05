package dto

type ReactFlowPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type ReactFlowNodeDTO struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Position   ReactFlowPosition      `json:"position"`
	Data       map[string]interface{} `json:"data"`
	DragHandle string                 `json:"dragHandle,omitempty"`
}

type ReactFlowEdgeDTO struct {
	ID           string                 `json:"id"`
	Source       string                 `json:"source"`
	Target       string                 `json:"target"`
	SourceHandle *string                `json:"sourceHandle"`
	TargetHandle *string                `json:"targetHandle"`
	Type         string                 `json:"type,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Animated     bool                   `json:"animated,omitempty"`
}

type SaveWorkflowRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type SaveWorkflowGraphRequest struct {
	Nodes []ReactFlowNodeDTO `json:"nodes" binding:"required"`
	Edges []ReactFlowEdgeDTO `json:"edges" binding:"required"`
}

type WorkflowResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Status      string            `json:"status"`
	Graph       ReactFlowGraphDTO `json:"graph"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

type ReactFlowGraphDTO struct {
	Nodes    []ReactFlowNodeDTO     `json:"nodes"`
	Edges    []ReactFlowEdgeDTO     `json:"edges"`
	Viewport map[string]interface{} `json:"viewport,omitempty"`
}
