//go:build mutation

// SPDX-License-Identifier: MIT

package core_test

// Run the mutation suite with:
//
//	go test -tags=mutation ./pkg/core -run TestMutations

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const mutationTimeout = 180 * time.Second

type mutationReplacement struct {
	old string
	new string
}

type mutation struct {
	id           string
	file         string
	replacements []mutationReplacement
	packages     []string
}

var mutations = []mutation{
	{
		id:   "zero-sum",
		file: "pkg/core/tx.go",
		replacements: []mutationReplacement{
			{old: "if !sum.IsZero() {", new: "if !sum.IsZero() && false {"},
		},
		packages: []string{"./pkg/core"},
	},
	{
		id:   "currency-consistency",
		file: "pkg/core/tx.go",
		replacements: []mutationReplacement{
			{
				old: "if entry.FunctionalCurrency() != b.config.FunctionalCurrency {",
				new: "if entry.FunctionalCurrency() != b.config.FunctionalCurrency && false {",
			},
		},
		packages: []string{"./pkg/core"},
	},
	{
		id:   "period-closure",
		file: "pkg/core/tx.go",
		replacements: []mutationReplacement{
			{old: "if effectiveDate.After(b.config.ClosedDate) {", new: "if effectiveDate.After(b.config.ClosedDate) || true {"},
		},
		packages: []string{"./pkg/core"},
	},
	{
		id:   "rate-positivity",
		file: "pkg/core/rates.go",
		replacements: []mutationReplacement{
			{
				old: "if entry.ExchangeRate().LessThanOrEqual(decimal.Zero) {",
				new: "if entry.ExchangeRate().LessThanOrEqual(decimal.Zero) && false {",
			},
		},
		packages: []string{"./pkg/core"},
	},
	{
		id:   "rate-tolerance",
		file: "pkg/core/rates.go",
		replacements: []mutationReplacement{
			{old: "if deviation.GreaterThan(tolerance) {", new: "if deviation.GreaterThan(tolerance) && false {"},
		},
		packages: []string{"./pkg/core"},
	},
	{
		id:   "money-precision",
		file: "pkg/currency/money.go",
		replacements: []mutationReplacement{
			{old: "if amount.Exponent() < -minorUnits {", new: "if amount.Exponent() < -minorUnits && false {"},
		},
		packages: []string{"./pkg/currency"},
	},
	{
		id:   "entries-copy",
		file: "pkg/core/tx.go",
		replacements: []mutationReplacement{
			{old: "copy(entriesCopy, t.entries)", new: "entriesCopy = t.entries"},
		},
		packages: []string{"./pkg/core"},
	},
	{
		id:   "metadata-copy",
		file: "pkg/core/tx.go",
		replacements: []mutationReplacement{
			{old: "return copyMetadata(t.metadata)", new: "return t.metadata"},
		},
		packages: []string{"./pkg/core"},
	},
	{
		id:   "rate-cache",
		file: "pkg/core/rates.go",
		replacements: []mutationReplacement{
			{
				old: "if providerRate, cached := cache[key]; cached {",
				new: "if providerRate, cached := cache[key]; cached && false {",
			},
		},
		packages: []string{"./pkg/core"},
	},
	{
		id:   "float-multiply",
		file: "pkg/core/entry.go",
		replacements: []mutationReplacement{
			{
				old: "txAmount.Mul(exchangeRate)",
				new: "decimal.NewFromFloat(float64(txAmount.InexactFloat64()) * exchangeRate.InexactFloat64())",
			},
			{
				old: "e.txAmount.Mul(e.exchangeRate)",
				new: "decimal.NewFromFloat(float64(e.txAmount.InexactFloat64()) * e.exchangeRate.InexactFloat64())",
			},
		},
		packages: []string{"./pkg/core"},
	},
}

func TestMutations(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	caught, survived, invalid := 0, 0, 0
	for _, candidate := range mutations {
		candidate := candidate
		t.Run(candidate.id, func(t *testing.T) {
			caseRoot := t.TempDir()
			copyMutationInputs(t, repoRoot, caseRoot)
			applyMutation(t, caseRoot, candidate)

			compileArgs := []string{"test", "-run", "^$", "./pkg/...", "./internal/..."}
			compileOutput, compileErr, compileTimedOut := runGoCommand(t, caseRoot, compileArgs)
			if compileTimedOut {
				invalid++
				t.Errorf("%s INVALID: compile check timed out\ncommand: go %s\n%s",
					candidate.id, strings.Join(compileArgs, " "), shortFailureOutput(compileOutput))
				return
			}
			if compileErr != nil {
				invalid++
				t.Errorf("%s INVALID: compile check failed\ncommand: go %s\n%s",
					candidate.id, strings.Join(compileArgs, " "), shortFailureOutput(compileOutput))
				return
			}

			testArgs := append([]string{"test", "-count=1"}, candidate.packages...)
			testOutput, testErr, testTimedOut := runGoCommand(t, caseRoot, testArgs)
			switch {
			case testTimedOut:
				invalid++
				t.Errorf("%s INVALID: test command timed out\ncommand: go %s\n%s",
					candidate.id, strings.Join(testArgs, " "), shortFailureOutput(testOutput))
			case testErr != nil:
				caught++
				t.Logf("%s CAUGHT\ncommand: go %s\n%s",
					candidate.id, strings.Join(testArgs, " "), shortFailureOutput(testOutput))
			default:
				survived++
				t.Errorf("%s SURVIVED\ncommand: go %s\n%s",
					candidate.id, strings.Join(testArgs, " "), shortFailureOutput(testOutput))
			}
		})
	}

	if invalid != 0 || caught < 9 {
		t.Fatalf("caught=%d survived=%d invalid=%d; require invalid=0 and caught>=9",
			caught, survived, invalid)
	}
	t.Logf("caught=%d survived=%d invalid=%d", caught, survived, invalid)
}

func shortFailureOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	const maxLength = 2000
	if len(trimmed) > maxLength {
		return trimmed[:maxLength] + "\n... (truncated)"
	}
	return trimmed
}

func copyMutationInputs(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	for _, name := range []string{"go.mod", "go.sum", "README.md"} {
		copyMutationFile(t, filepath.Join(sourceRoot, name), filepath.Join(destinationRoot, name))
	}
	for _, dir := range []string{"pkg", "internal"} {
		copyMutationTree(t, filepath.Join(sourceRoot, dir), filepath.Join(destinationRoot, dir))
	}
}

func copyMutationTree(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(destinationRoot, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		copyMutationFile(t, path, destination)
		return nil
	})
	if err != nil {
		t.Fatalf("copy %s: %v", sourceRoot, err)
	}
}

func copyMutationFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source) //nolint:gosec // repository fixture path
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(destination), err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil { //nolint:gosec // temp mutation fixture
		t.Fatalf("write %s: %v", destination, err)
	}
}

func applyMutation(t *testing.T, caseRoot string, candidate mutation) {
	t.Helper()
	path := filepath.Join(caseRoot, candidate.file)
	data, err := os.ReadFile(path) //nolint:gosec // temp mutation fixture
	if err != nil {
		t.Fatalf("read mutation target: %v", err)
	}
	text := string(data)
	for _, replacement := range candidate.replacements {
		if !strings.Contains(text, replacement.old) {
			t.Fatalf("%s target not found in %s: %q", candidate.id, candidate.file, replacement.old)
		}
		text = strings.ReplaceAll(text, replacement.old, replacement.new)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write mutation target: %v", err)
	}
}

func runGoCommand(t *testing.T, caseRoot string, args []string) (string, error, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), mutationTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...) //nolint:gosec // fixed local test command
	cmd.Dir = caseRoot
	cmd.Env = append(os.Environ(), "GOPROXY=off")
	output, err := cmd.CombinedOutput()
	timedOut := ctx.Err() == context.DeadlineExceeded
	return string(output), err, timedOut
}
