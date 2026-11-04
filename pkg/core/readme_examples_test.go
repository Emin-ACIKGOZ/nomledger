// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	readmeBuildTimeout = 60 * time.Second
	readmeFence        = "```"
)

func isGoFenceInfo(info string) bool {
	fields := strings.Fields(info)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "go", "golang":
		return true
	default:
		return false
	}
}

func extractReadmeGoBlocks(content string) ([]string, error) {
	lines := strings.Split(content, "\n")
	var blocks []string
	var current []string
	inBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock && strings.HasPrefix(trimmed, readmeFence) {
			info := strings.TrimSpace(strings.TrimPrefix(trimmed, readmeFence))
			if isGoFenceInfo(info) {
				inBlock = true
				current = nil
			}
			continue
		}
		if inBlock && trimmed == readmeFence {
			blocks = append(blocks, strings.Join(current, "\n"))
			inBlock = false
			continue
		}
		if inBlock {
			current = append(current, line)
		}
	}
	if inBlock {
		return nil, errors.New("README contains an unterminated Go code block")
	}
	return blocks, nil
}

func TestReadmeExtractorRecognizesGoAliases(t *testing.T) {
	content := strings.Join([]string{
		"```go",
		"package goexample",
		readmeFence,
		"",
		"```golang",
		"package golanexample",
		readmeFence,
		"",
		"```Go",
		"package capitalexample",
		readmeFence,
		"",
		"```go.mod",
		"module nope",
		readmeFence,
		"",
		"```bash",
		"echo no",
		readmeFence,
	}, "\n")

	blocks, err := extractReadmeGoBlocks(content)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(blocks) != 3 {
		t.Fatalf("blocks = %d; want 3", len(blocks))
	}

	joined := strings.Join(blocks, "\n")
	for _, want := range []string{"package goexample", "package golanexample", "package capitalexample"} {
		if !strings.Contains(joined, want) {
			t.Errorf("extracted blocks are missing %q", want)
		}
	}
	for _, unwanted := range []string{"module nope", "echo no"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("extracted blocks incorrectly include %q", unwanted)
		}
	}
}

func TestReadmeExtractorIgnoresNonGoBlocks(t *testing.T) {
	content := "```go.mod\nmodule nope\n```\n\n```bash\necho no\n```\n"
	blocks, err := extractReadmeGoBlocks(content)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(blocks) != 0 {
		t.Fatalf("blocks = %d; want 0", len(blocks))
	}
}

func TestReadmeExtractorRejectsUnterminatedGoBlock(t *testing.T) {
	if _, err := extractReadmeGoBlocks("```golang\npackage broken\n"); err == nil {
		t.Fatal("unterminated Go block accepted; want error")
	}
}

func TestReadmeExamplesCompile(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}

	readmePath := filepath.Join("..", "..", "README.md")
	content, err := os.ReadFile(readmePath) //nolint:gosec // fixed repository README path
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	blocks, err := extractReadmeGoBlocks(string(content))
	if err != nil {
		t.Fatalf("extract README Go blocks: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("README contains no recognized Go code blocks")
	}

	repoRoot, goSum := readmeBuildFixture(t)
	for index, block := range blocks {
		t.Run(fmt.Sprintf("block_%d", index+1), func(t *testing.T) {
			buildReadmeBlock(t, repoRoot, goSum, block, index+1)
		})
	}
}

// TestInvalidGolangBlockIsCompiled proves golang aliases are not silently
// skipped: an invalid ```golang block is extracted and then fails to build.
func TestInvalidGolangBlockIsCompiled(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}

	content := "```golang\npackage main\n\nfunc main() { this is not valid go }\n```\n"
	blocks, err := extractReadmeGoBlocks(content)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("blocks = %d; want 1", len(blocks))
	}

	repoRoot, goSum := readmeBuildFixture(t)
	output, err := runReadmeBlockBuild(t, repoRoot, goSum, blocks[0])
	if err == nil {
		t.Fatalf("invalid golang block unexpectedly compiled:\n%s", output)
	}
}

func readmeBuildFixture(t *testing.T) (string, []byte) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	goSum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum")) //nolint:gosec // repository fixture path
	if err != nil {
		t.Fatalf("read go.sum: %v", err)
	}
	return repoRoot, goSum
}

func buildReadmeBlock(t *testing.T, repoRoot string, goSum []byte, block string, index int) {
	t.Helper()
	output, err := runReadmeBlockBuild(t, repoRoot, goSum, block)
	if err != nil {
		t.Fatalf("README Go block %d failed to build: %v\n%s", index, err, output)
	}
}

func runReadmeBlockBuild(t *testing.T, repoRoot string, goSum []byte, block string) ([]byte, error) {
	t.Helper()
	moduleDir := t.TempDir()
	relativeRoot, err := filepath.Rel(moduleDir, repoRoot)
	if err != nil {
		t.Fatalf("compute replace path: %v", err)
	}

	goMod := fmt.Sprintf(`module readmeexample

go 1.22

require github.com/Emin-ACIKGOZ/nomledger v0.0.0

replace github.com/Emin-ACIKGOZ/nomledger => %s
`, filepath.ToSlash(relativeRoot))

	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "go.sum"), goSum, 0o600); err != nil {
		t.Fatalf("write go.sum: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "main.go"), []byte(block), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), readmeBuildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", ".")
	cmd.Dir = moduleDir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod")

	return cmd.CombinedOutput()
}
