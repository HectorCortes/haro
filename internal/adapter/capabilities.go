package adapter

// Capabilities describes the adapter's supported features.
// The "_" prefix convention for namespaced extra keys is documented
// but not enforced; keys such as "_custom" are preserved as-is.
type Capabilities struct {
	ProtocolVersion int            `json:"protocolVersion"`
	Permission      bool           `json:"permission"`
	Terminal        bool           `json:"terminal"`
	LoadSession     bool           `json:"loadSession"`
	Extra           map[string]any `json:"extra,omitempty"`
}
