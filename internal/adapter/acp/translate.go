package acp

import (
	"github.com/HectorCortes/haro/internal/adapter"
)

// ToACP bijectively translates Capabilities to ACP map.
func ToACP(c adapter.Capabilities) map[string]any {
	m := map[string]any{
		"protocolVersion": c.ProtocolVersion,
		"permission":      c.Permission,
		"terminal":        c.Terminal,
		"loadSession":     c.LoadSession,
	}
	for k, v := range c.Extra {
		m[k] = v
	}
	return m
}

// FromACP translates ACP map back to Capabilities, preserving "_" keys.
func FromACP(m map[string]any) adapter.Capabilities {
	c := adapter.Capabilities{}
	if v, ok := m["protocolVersion"]; ok {
		switch vv := v.(type) {
		case int:
			c.ProtocolVersion = vv
		case float64:
			c.ProtocolVersion = int(vv)
		case int64:
			c.ProtocolVersion = int(vv)
		default:
			// try via type switch for json.Number? Already handled?
			if f, ok := v.(float64); ok {
				c.ProtocolVersion = int(f)
			}
		}
	}
	if v, ok := m["permission"]; ok {
		if b, ok := v.(bool); ok {
			c.Permission = b
		}
	}
	if v, ok := m["terminal"]; ok {
		if b, ok := v.(bool); ok {
			c.Terminal = b
		}
	}
	if v, ok := m["loadSession"]; ok {
		if b, ok := v.(bool); ok {
			c.LoadSession = b
		}
	}
	extra := make(map[string]any)
	for k, v := range m {
		if k == "protocolVersion" || k == "permission" || k == "terminal" || k == "loadSession" {
			continue
		}
		extra[k] = v
	}
	if len(extra) > 0 {
		c.Extra = extra
	}
	return c
}

// TranslateInitialize builds ACP initialize params from core capabilities.
func TranslateInitialize(core adapter.Capabilities) map[string]any {
	return map[string]any{
		"protocolVersion":  core.ProtocolVersion,
		"clientCapabilities": ToACP(core),
	}
}

// TranslateSessionNew builds session/new params.
func TranslateSessionNew(bundle adapter.SessionBundle) map[string]any {
	return map[string]any{
		"instructions":  bundle.Instructions,
		"workspaceRoot": bundle.WorkspaceRoot,
		"requires":      bundle.Requires,
	}
}

// TranslatePrompt builds session/prompt params.
func TranslatePrompt(sessionID string, input adapter.PromptInput) map[string]any {
	return map[string]any{
		"sessionId": sessionID,
		"text":      input.Text,
	}
}

// TranslateUpdate builds session/update notification params (harness -> broker).
func TranslateUpdate(sessionID string, cursor int, delta any) map[string]any {
	return map[string]any{
		"sessionId": sessionID,
		"cursor":    cursor,
		"delta":     delta,
	}
}

// TranslateCancel builds session/cancel params.
func TranslateCancel(sessionID string) map[string]any {
	return map[string]any{
		"sessionId": sessionID,
	}
}

// TranslateRequestPermission builds session/request_permission params (harness -> broker).
func TranslateRequestPermission(sessionID string, req adapter.PermissionRequest) map[string]any {
	return map[string]any{
		"sessionId":   sessionID,
		"kind":        req.Kind,
		"description": req.Description,
		"options":     req.Options,
	}
}

// TranslatePermissionDecision parses permission decision result.
func TranslatePermissionDecision(m map[string]any) adapter.PermissionDecision {
	if v, ok := m["option"].(string); ok {
		return adapter.PermissionDecision{Option: v}
	}
	return adapter.PermissionDecision{}
}
