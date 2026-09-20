package ipc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// socketHashLen is the bounded hash prefix length used for endpoint names.
// A fixed-width digest keeps the endpoint length independent of the project
// root depth, so UDS paths stay inside the 108 byte (NUL included) bound for
// any canonical root.
const socketHashLen = 16

// shortSocketHashLen is the fallback prefix length used when the default
// endpoint still exceeds the UDS bound (e.g. a very long TMPDIR).
const shortSocketHashLen = 8

// CanonicalRoot returns the symlink-resolved, cleaned form of root.
// Relative paths resolve against the current directory.
func CanonicalRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("empty project root")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("abs root: %w", err)
	}
	eval, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// A not-yet-created root (e.g. before haro init) still needs a
		// stable endpoint: fall back to the cleaned absolute path.
		if os.IsNotExist(err) {
			return filepath.Clean(abs), nil
		}
		return "", fmt.Errorf("eval symlinks %q: %w", abs, err)
	}
	return filepath.Clean(eval), nil
}

// SocketPath derives the per-project broker endpoint from the canonical
// project root: EvalSymlinks(Clean(root)) -> sha256 short hex. Unix uses
// os.TempDir()/haro-<hash>.sock (NUL included <= 108 bytes); Windows uses
// \\.\pipe\haro-<hash>. Equivalent inputs (symlinks, trailing separators)
// share one endpoint; distinct roots differ.
func SocketPath(root string) (string, error) {
	canon, err := CanonicalRoot(root)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(canon))
	fullHash := hex.EncodeToString(sum[:])
	if runtime.GOOS == "windows" {
		return `\\.\pipe\haro-` + fullHash[:socketHashLen], nil
	}

	start := socketHashLen
	if len(filepath.Join(os.TempDir(), "haro-"+fullHash[:start]+".sock"))+1 > 108 {
		start = shortSocketHashLen
	}
	for prefixLen := start; prefixLen <= len(fullHash); prefixLen += 8 {
		p := filepath.Join(os.TempDir(), "haro-"+fullHash[:prefixLen]+".sock")
		if len(p)+1 > 108 {
			continue
		}
		occupied, reusable := socketEndpointState(p)
		if !occupied || reusable {
			return p, nil
		}
	}
	return "", fmt.Errorf("socket path collision for %q", canon)
}

// socketEndpointState reports whether a Unix endpoint exists and whether it
// has the metadata of a broker-owned endpoint. Foreign files and endpoints
// with non-private permissions are never reused; SocketPath extends the hash
// prefix instead. Existing private sockets remain stable for live daemons and
// safe stale-endpoint cleanup.
func socketEndpointState(path string) (occupied, reusable bool) {
	info, err := os.Stat(path)
	if err != nil {
		return !os.IsNotExist(err), false
	}
	return true, info.Mode().Perm() == 0o600 && (info.Mode().IsRegular() || info.Mode()&os.ModeSocket != 0)
}
