package support

type TypeScriptTaintModel struct {
	Sources []TypeScriptTaintSource `json:"sources"`
	Sinks   []TypeScriptTaintSink   `json:"sinks"`
}

type TypeScriptTaintSource struct {
	ID                string   `json:"id"`
	Label             string   `json:"label"`
	Kind              string   `json:"kind"`
	BaseIdentifiers   []string `json:"base_identifiers,omitempty"`
	BasePropertyNames []string `json:"base_property_names,omitempty"`
	CallMembers       []string `json:"call_members,omitempty"`
	ReceiverTypeNames []string `json:"receiver_type_names,omitempty"`
}

type TypeScriptTaintSink struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Kind            string `json:"kind"`
	Module          string `json:"module,omitempty"`
	Member          string `json:"member,omitempty"`
	PropertyName    string `json:"property_name,omitempty"`
	ArgumentIndexes []int  `json:"argument_indexes,omitempty"`
}
