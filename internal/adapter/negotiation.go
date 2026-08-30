package adapter

import (
	"fmt"
)

// Negotiate performs bilateral capability negotiation.
// It returns the intersection of boolean capabilities and the union of Extra keys.
// Additive Extra keys do not require a major version bump.
// ProtocolVersion mismatch is allowed only if additive; otherwise returns error.
// For this slice, we require ProtocolVersion equality (no major incompatibility).
func Negotiate(core, agent Capabilities) (Capabilities, error) {
	// For now, require same ProtocolVersion; future major bumps only for mandatory incompatibility
	if core.ProtocolVersion != agent.ProtocolVersion {
		return Capabilities{}, fmt.Errorf("protocol version mismatch: core %d vs agent %d", core.ProtocolVersion, agent.ProtocolVersion)
	}
	neg := Capabilities{
		ProtocolVersion: core.ProtocolVersion,
		Permission:      core.Permission && agent.Permission,
		Terminal:        core.Terminal && agent.Terminal,
		LoadSession:     core.LoadSession && agent.LoadSession,
		Extra:           make(map[string]any),
	}
	// Union of Extra: agent wins on conflict, but both additive keys preserved
	for k, v := range core.Extra {
		neg.Extra[k] = v
	}
	for k, v := range agent.Extra {
		neg.Extra[k] = v
	}
	if len(neg.Extra) == 0 {
		neg.Extra = nil
	}
	return neg, nil
}

// CheckCapability gates optional methods.
// Returns ErrUnsupportedCapability if the capability is not announced.
// Unknown "_" prefixed extra capabilities are considered additive and not gated.
func CheckCapability(caps Capabilities, method string) error {
	switch method {
	case "Terminal":
		if !caps.Terminal {
			return ErrUnsupportedCapability
		}
	case "LoadSession", "LoadPrevious":
		if !caps.LoadSession {
			return ErrUnsupportedCapability
		}
	case "Permission", "RequestPermission":
		if !caps.Permission {
			return ErrUnsupportedCapability
		}
	default:
		// Additive unknown capabilities are allowed
		if len(method) > 0 && method[0] == '_' {
			return nil
		}
		// For any other known optional, default to not gated? Treat as allowed.
		// But for strictness, if method is not recognized as mandatory, allow additive.
		return nil
	}
	return nil
}
