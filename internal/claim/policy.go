package claim

import (
	"path/filepath"
	"strings"
)

// isShared reports whether mode is shared. Isolated modes have prefix "isolated".
func isShared(mode string) bool {
	if mode == "shared" {
		return true
	}
	if strings.HasPrefix(mode, "isolated") {
		return false
	}
	// unknown modes treat as shared for safety
	return true
}

// ShouldBlock implements the 4-row matrix.
// existing/incoming are modes ("isolated:execID" or "shared").
// onConflict is "block" or "allow" (default block).
// isExternal forces shared treatment.
func ShouldBlock(existingMode, incomingMode, onConflict string, isExternal bool) bool {
	if isExternal {
		return true // external always shared -> block
	}
	exShared := isShared(existingMode)
	inShared := isShared(incomingMode)
	// isolated/isolated -> allow
	if !exShared && !inShared {
		return false
	}
	// shared/shared -> block
	if exShared && inShared {
		return true
	}
	// mixed
	if onConflict == "allow" {
		return false
	}
	return true
}

// IsExternal reports whether logicalPath is under any external prefix.
// externalPrefixes are canonical logical prefixes (clean, slash-normalized).
func IsExternal(logicalPath string, externalPrefixes []string) bool {
	if len(externalPrefixes) == 0 {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(logicalPath))
	if clean == "." {
		clean = ""
	}
	for _, p := range externalPrefixes {
		pp := filepath.ToSlash(filepath.Clean(p))
		if pp == "." {
			pp = ""
		}
		if pp == "" {
			continue
		}
		if clean == pp || strings.HasPrefix(clean, pp+"/") {
			return true
		}
	}
	return false
}
