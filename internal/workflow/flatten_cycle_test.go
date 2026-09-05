package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFlatten_Cycle(t *testing.T) {
	t.Run("direct self cycle", func(t *testing.T) {
		root := t.TempDir()
		workflowsBase := filepath.Join(root, ".haro", "workflows")
		mainPath := filepath.Join(workflowsBase, "main", "workflow.yaml")
		// main includes itself
		files := map[string][]byte{
			mainPath: []byte("version: 2\nname: main\nsteps:\n  - id: self\n    type: workflow\n    source: ./workflow.yaml\n"),
		}
		readFile := func(p string) ([]byte, error) {
			if data, ok := files[p]; ok {
				return data, nil
			}
			// also handle canonical realpath after Clean; just return
			return nil, nil
		}
		// Need to handle path resolution: mainPath's dir is workflows/main, source ./workflow.yaml resolves to same file
		_, err := Flatten(mainPath, readFile)
		if err == nil {
			t.Fatalf("expected cycle_detected")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Code != "cycle_detected" {
			t.Fatalf("expected ValidationError cycle_detected, got %v", err)
		}
	})

	t.Run("transitive cycle via renamed nodes", func(t *testing.T) {
		root := t.TempDir()
		base := filepath.Join(root, ".haro", "workflows")
		mainPath := filepath.Join(base, "main", "workflow.yaml")
		libAPath := filepath.Join(base, "lib", "a.yaml")
		libBPath := filepath.Join(base, "lib", "b.yaml")
		files := map[string][]byte{
			mainPath: []byte("version: 2\nname: main\nsteps:\n  - id: a_node\n    type: workflow\n    source: ../lib/a.yaml\n"),
			libAPath: []byte("version: 2\nname: a\nsteps:\n  - id: b_node\n    type: workflow\n    source: ./b.yaml\n"),
			libBPath: []byte("version: 2\nname: b\nsteps:\n  - id: back\n    type: workflow\n    source: ../main/workflow.yaml\n"),
		}
		readFile := func(p string) ([]byte, error) {
			clean := filepath.Clean(p)
			if data, ok := files[clean]; ok {
				return data, nil
			}
			return nil, nil
		}
		_, err := Flatten(mainPath, readFile)
		if err == nil {
			t.Fatalf("expected transitive cycle")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Code != "cycle_detected" {
			t.Fatalf("expected cycle_detected got %v", err)
		}
	})
}

func TestFlatten_Guards(t *testing.T) {
	t.Run("depth exceeds 16", func(t *testing.T) {
		fsRoot := t.TempDir()
		fsBase := filepath.Join(fsRoot, ".haro", "workflows")
		fsMain := filepath.Join(fsBase, "main", "workflow.yaml")
		// create chain depth 17 on disk
		cur := fsMain
		for i := 0; i < 17; i++ {
			dir := filepath.Dir(cur)
			// need to create dir
			_ = filepath.Join(dir, "..")
			if err := mkDirs(dir); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			next := filepath.Join(fsBase, "depth", "file"+itoa(i)+".yaml")
			if i == 16 {
				// final leaf
				writeFile(t, cur, "version: 2\nname: x\nsteps:\n  - id: w\n    type: workflow\n    source: "+relPath(cur, next)+"\n")
				writeFile(t, next, "version: 2\nname: leaf\nsteps:\n  - id: leaf\n    type: command\n    run: echo hi\n")
				break
			}
			writeFile(t, cur, "version: 2\nname: x\nsteps:\n  - id: w\n    type: workflow\n    source: "+relPath(cur, next)+"\n")
			cur = next
		}
		_, err := Flatten(fsMain, nil)
		if err == nil {
			t.Fatalf("expected guard_exceeded depth")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Code != "guard_exceeded" {
			t.Fatalf("expected guard_exceeded got %v", err)
		}
	})

	t.Run("expansions exceeds 256", func(t *testing.T) {
		root := t.TempDir()
		base := filepath.Join(root, ".haro", "workflows")
		mainPath := filepath.Join(base, "main", "workflow.yaml")
		libPath := filepath.Join(base, "lib", "workflow.yaml")
		yaml := "version: 2\nname: main\nsteps:\n"
		for i := 0; i < 257; i++ {
			yaml += "  - id: w" + itoa(i) + "\n    type: workflow\n    source: ../lib/workflow.yaml\n"
		}
		files := map[string][]byte{
			filepath.Clean(mainPath): []byte(yaml),
			filepath.Clean(libPath):  []byte("version: 2\nname: lib\nsteps:\n  - id: s\n    type: command\n    run: echo hi\n"),
		}
		readFile := func(p string) ([]byte, error) {
			if d, ok := files[filepath.Clean(p)]; ok {
				return d, nil
			}
			return nil, nil
		}
		_, err := Flatten(mainPath, readFile)
		if err == nil {
			t.Fatalf("expected guard_exceeded expansions")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Code != "guard_exceeded" {
			t.Fatalf("expected guard_exceeded got %v", err)
		}
	})

	t.Run("flattened steps exceeds 256", func(t *testing.T) {
		root := t.TempDir()
		base := filepath.Join(root, ".haro", "workflows")
		mainPath := filepath.Join(base, "main", "workflow.yaml")
		libPath := filepath.Join(base, "lib", "workflow.yaml")
		yamlLib := "version: 2\nname: lib\nsteps:\n"
		for i := 0; i < 257; i++ {
			yamlLib += "  - id: s" + itoa(i) + "\n    type: command\n    run: echo hi\n"
		}
		files := map[string][]byte{
			filepath.Clean(mainPath): []byte("version: 2\nname: main\nsteps:\n  - id: w\n    type: workflow\n    source: ../lib/workflow.yaml\n"),
			filepath.Clean(libPath):  []byte(yamlLib),
		}
		readFile := func(p string) ([]byte, error) {
			if d, ok := files[filepath.Clean(p)]; ok {
				return d, nil
			}
			return nil, nil
		}
		_, err := Flatten(mainPath, readFile)
		if err == nil {
			t.Fatalf("expected guard_exceeded steps")
		}
		if ve, ok := err.(*ValidationError); !ok || ve.Code != "guard_exceeded" {
			t.Fatalf("expected guard_exceeded got %v", err)
		}
	})
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

func mkDirs(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func relPath(from, to string) string {
	rel, err := filepath.Rel(filepath.Dir(from), to)
	if err != nil {
		return to
	}
	return rel
}
