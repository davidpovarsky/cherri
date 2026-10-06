package ir

// NativeActionNode represents a single physical Apple Shortcut action in the pipeline.
type NativeActionNode struct {
	NodeID             string                 `json:"nodeId"`
	AppleIdentifier    string                 `json:"appleIdentifier"`
	Parameters         map[string]interface{} `json:"parameters"`
	OutputUUID         string                 `json:"outputUUID,omitempty"`
	OutputName         string                 `json:"outputName,omitempty"`
	GroupingIdentifier string                 `json:"groupingIdentifier,omitempty"`
	ControlFlowMode    int                    `json:"controlFlowMode,omitempty"` // 0=begin, 1=else/case, 2=end
}

// AttachmentToken represents an inline variable/action output attachment in text.
type AttachmentToken struct {
	Type          string                 `json:"type"` // ActionOutput, Variable, ExtensionInput, etc.
	OutputUUID    string                 `json:"outputUUID,omitempty"`
	OutputName    string                 `json:"outputName,omitempty"`
	Aggrandizements []map[string]interface{} `json:"aggrandizements,omitempty"`
}

// NativeWorkflow represents an entire Apple Shortcut workflow ready for plist serialization.
type NativeWorkflow struct {
	ClientVersion      string                   `json:"clientVersion"`
	WorkflowTypes           []string                 `json:"workflowTypes,omitempty"`
	InputContentItemClasses []string                 `json:"inputContentItemClasses,omitempty"`
	IconGlyph               int                      `json:"iconGlyph,omitempty"`
	IconColor          int                      `json:"iconColor,omitempty"`
	Actions            []*NativeActionNode      `json:"actions"`
	ImportQuestions    []map[string]interface{} `json:"importQuestions,omitempty"`
	HasExplicitReturn  bool                     `json:"hasExplicitReturn"`
	IsFullNativePass   bool                     `json:"isFullNativePass"`
	NativeWorkflowRaw  map[string]interface{}   `json:"nativeWorkflowRaw,omitempty"`
}

// NewNativeWorkflow creates an initialized NativeWorkflow.
func NewNativeWorkflow() *NativeWorkflow {
	return &NativeWorkflow{
		ClientVersion: "4711",
		Actions:       make([]*NativeActionNode, 0),
	}
}

// AddAction appends an action node to the workflow.
func (w *NativeWorkflow) AddAction(node *NativeActionNode) {
	w.Actions = append(w.Actions, node)
}
