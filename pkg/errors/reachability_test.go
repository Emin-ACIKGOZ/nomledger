// SPDX-License-Identifier: MIT

package errors_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestSentinelReachability: every exported sentinel must be referenced
// by non-test production code outside the entire pkg/errors package.
func TestSentinelReachability(t *testing.T) {
	declared := declaredSentinelNames(t, "errors.go")
	if len(declared) == 0 {
		t.Fatal("no sentinels found in errors.go")
	}
	sentinels := make(map[string]bool, len(declared))
	for _, name := range declared {
		sentinels[name] = true
	}

	referenced, err := scanReachableReferences(t, "..", filepath.Join("..", "errors"), sentinels)
	if err != nil {
		t.Fatalf("scan pkg tree: %v", err)
	}

	var unused []string
	for _, name := range declared {
		if !referenced[name] {
			unused = append(unused, name)
		}
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		t.Fatalf("unused sentinels outside pkg/errors: %v", unused)
	}
	t.Logf("all %d sentinels referenced outside pkg/errors", len(declared))
}

// TestReachabilityExcludesErrorsPackage proves references inside pkg/errors do
// not count as production reachability.
func TestReachabilityExcludesErrorsPackage(t *testing.T) {
	root := t.TempDir()
	errorsDir := filepath.Join(root, "pkg", "errors")
	writeSyntheticFile(t, filepath.Join(errorsDir, "errors.go"), "package errors\n\nvar ErrOnlyInternal = 1\nvar ErrUsedElsewhere = 2\n")
	writeSyntheticFile(t, filepath.Join(errorsDir, "internal.go"), "package errors\n\nvar _ = ErrOnlyInternal\n")
	writeSyntheticFile(t, filepath.Join(root, "pkg", "other", "use.go"), "package other\n\nvar _ = ErrUsedElsewhere\n")
	writeSyntheticFile(t, filepath.Join(root, "pkg", "other", "use_test.go"), "package other\n\nvar _ = ErrOnlyInternal\n")

	sentinels := map[string]bool{
		"ErrOnlyInternal":  true,
		"ErrUsedElsewhere": true,
	}
	referenced, err := scanReachableReferences(t, root, errorsDir, sentinels)
	if err != nil {
		t.Fatalf("scan synthetic tree: %v", err)
	}

	if referenced["ErrOnlyInternal"] {
		t.Error("sentinel referenced only inside pkg/errors was counted as reachable")
	}
	if !referenced["ErrUsedElsewhere"] {
		t.Error("sentinel referenced in another package was not counted as reachable")
	}
}

func scanReachableReferences(
	t *testing.T,
	root string,
	excludedDir string,
	sentinels map[string]bool,
) (map[string]bool, error) {
	t.Helper()
	referenced := make(map[string]bool, len(sentinels))
	excluded := filepath.Clean(excludedDir)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if filepath.Clean(path) == excluded {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if ok && sentinels[ident.Name] {
				referenced[ident.Name] = true
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return referenced, nil
}

func writeSyntheticFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
