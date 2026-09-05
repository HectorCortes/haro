package claim

import "testing"

func TestPolicy(t *testing.T) {
	tests := []struct {
		name       string
		existing   string
		incoming   string
		onConflict string
		isExternal bool
		wantBlock  bool
	}{
		{"isolated/isolated allow", "isolated:exec1", "isolated:exec2", "block", false, false},
		{"shared/shared block", "shared", "shared", "block", false, true},
		{"isolated/shared block default", "isolated:exec1", "shared", "block", false, true},
		{"isolated/shared allow explicit", "isolated:exec1", "shared", "allow", false, false},
		{"shared/isolated block default symmetric", "shared", "isolated:exec2", "block", false, true},
		{"shared/isolated allow explicit symmetric", "shared", "isolated:exec2", "allow", false, false},
		{"external forces shared block", "isolated:exec1", "isolated:exec2", "block", true, true},
		{"external shared/shared block", "shared", "shared", "block", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldBlock(tt.existing, tt.incoming, tt.onConflict, tt.isExternal)
			if got != tt.wantBlock {
				t.Fatalf("ShouldBlock(%q,%q,%q,%v)=%v want %v", tt.existing, tt.incoming, tt.onConflict, tt.isExternal, got, tt.wantBlock)
			}
		})
	}
}

func TestIsExternal(t *testing.T) {
	external := []string{"cache", ".tmp/cache"}
	tests := []struct {
		path string
		want bool
	}{
		{"cache/file.ts", true},
		{"cache", true},
		{".tmp/cache/foo", true},
		{"src/foo.ts", false},
		{"cache2/file", false},
		{"cachefoo", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsExternal(tt.path, external)
			if got != tt.want {
				t.Fatalf("IsExternal(%q)=%v want %v", tt.path, got, tt.want)
			}
		})
	}
	t.Run("empty external", func(t *testing.T) {
		if IsExternal("cache/file", nil) {
			t.Fatalf("empty external should not block")
		}
		if IsExternal("cache/file", []string{}) {
			t.Fatalf("empty slice should not block")
		}
	})
}

func TestIsExternalBoundary(t *testing.T) {
	// ensure boundary: cache should not match cachefoo
	if IsExternal("cachefoo/bar", []string{"cache"}) {
		t.Fatalf("cache should not match cachefoo")
	}
	if !IsExternal("cache/bar", []string{"cache"}) {
		t.Fatalf("cache should match cache/bar")
	}
}
