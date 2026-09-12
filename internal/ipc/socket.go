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
	hash := hex.EncodeToString(sum[:])[:socketHashLen]
	if runtime.GOOS == "windows" {
		return `\\.\pipe\haro-` + hash, nil
	}
	p := filepath.Join(os.TempDir(), "haro-"+hash+".sock")
	// Bound enforcement: UDS paths are limited to 108 bytes including NUL.
	// Shorten the hash prefix only while the bound is violated; the hash is
	// already bounded so this normally never triggers.
	if len(p)+1 > 108 {
		p = filepath.Join(os.TempDir(), "haro-"+hex.EncodeToString(sum[:])[:shortSocketHashLen]+".sock")
	}
	if len(p)+1 > 108 {
		return "", fmt.Errorf("socket path %q exceeds 108 bytes incl NUL", p)
	}
	return p, nil
}
