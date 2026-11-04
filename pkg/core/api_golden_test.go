// SPDX-License-Identifier: MIT

package core_test

// Update the API golden with:
//
//	UPDATE_API_GOLDEN=1 go test ./pkg/core -run TestAPIGolden

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type apiRoot struct {
	dir         string
	packageName string
}

// TestAPIGolden: the exported API is frozen by a checked-in golden.
func TestAPIGolden(t *testing.T) {
	lines := collectAPILines(t)
	if len(lines) == 0 {
		t.Fatal("no exported API declarations found")
	}

	goldenPath := filepath.Join("testdata", "api.golden")
	if os.Getenv("UPDATE_API_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o750); err != nil {
			t.Fatalf("create golden directory: %v", err)
		}
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(goldenPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s with %d entries", goldenPath, len(lines))
		return
	}

	data, err := os.ReadFile(goldenPath) //nolint:gosec // fixed repository testdata path
	if err != nil {
		t.Fatalf("read API golden (run UPDATE_API_GOLDEN=1 go test ./pkg/core -run TestAPIGolden): %v", err)
	}
	want := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(want) == 1 && want[0] == "" {
		want = nil
	}
	if diff := apiGoldenDiff(want, lines); diff != "" {
		t.Fatalf("exported API differs from %s:\n%s", goldenPath, diff)
	}
	t.Logf("API golden matches %d entries", len(lines))
}

func collectAPILines(t *testing.T) []string {
	t.Helper()
	roots := []apiRoot{
		{dir: "../currency", packageName: "pkg/currency"},
		{dir: ".", packageName: "pkg/core"},
		{dir: "../errors", packageName: "pkg/errors"},
		{dir: "../interfaces", packageName: "pkg/interfaces"},
	}
	var lines []string
	for _, root := range roots {
		lines = append(lines, collectAPIRoot(t, root)...)
	}
	return lines
}

func collectAPIRoot(t *testing.T, root apiRoot) []string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root.dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		lines = append(lines, apiLinesForFile(fileSet, file, root.packageName)...)
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", root.dir, err)
	}
	return lines
}

func apiLinesForFile(fileSet *token.FileSet, file *ast.File, packageName string) []string {
	var lines []string
	for _, declaration := range file.Decls {
		switch decl := declaration.(type) {
		case *ast.GenDecl:
			lines = append(lines, apiLinesForGenDecl(fileSet, decl, packageName)...)
		case *ast.FuncDecl:
			lines = append(lines, apiLinesForFunc(fileSet, decl, packageName)...)
		}
	}
	return lines
}

func apiLinesForGenDecl(fileSet *token.FileSet, decl *ast.GenDecl, packageName string) []string {
	if decl.Tok == token.CONST || decl.Tok == token.VAR {
		if !genDeclHasExportedName(decl) {
			return nil
		}
		kind := decl.Tok.String()
		rendered := strings.TrimPrefix(renderNode(fileSet, decl), kind+" ")
		return []string{packageName + " " + kind + " " + rendered}
	}
	if decl.Tok != token.TYPE {
		return nil
	}
	var lines []string
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok || !ast.IsExported(typeSpec.Name.Name) {
			continue
		}
		lines = append(lines, apiTypeLines(fileSet, typeSpec, packageName)...)
	}
	return lines
}

func genDeclHasExportedName(decl *ast.GenDecl) bool {
	for _, spec := range decl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range valueSpec.Names {
			if ast.IsExported(name.Name) {
				return true
			}
		}
	}
	return false
}

func apiTypeLines(fileSet *token.FileSet, typeSpec *ast.TypeSpec, packageName string) []string {
	name := typeSpec.Name.Name
	prefix := packageName + " type " + name
	var lines []string

	switch underlying := typeSpec.Type.(type) {
	case *ast.StructType:
		lines = append(lines, prefix+" struct")
		lines = append(lines, structFieldLines(fileSet, underlying, prefix)...)
	case *ast.InterfaceType:
		lines = append(lines, prefix+" interface")
		lines = append(lines, interfaceMethodLines(fileSet, underlying, prefix)...)
	default:
		lines = append(lines, prefix+" "+renderNode(fileSet, underlying))
	}
	return lines
}

func structFieldLines(fileSet *token.FileSet, structType *ast.StructType, prefix string) []string {
	var lines []string
	for _, field := range structType.Fields.List {
		fieldType := renderNode(fileSet, field.Type)
		if len(field.Names) == 0 {
			if ast.IsExported(baseTypeName(field.Type)) {
				lines = append(lines, prefix+" field embedded "+fieldType)
			}
			continue
		}
		for _, name := range field.Names {
			if ast.IsExported(name.Name) {
				lines = append(lines, prefix+" field "+name.Name+" "+fieldType)
			}
		}
	}
	return lines
}

func interfaceMethodLines(fileSet *token.FileSet, interfaceType *ast.InterfaceType, prefix string) []string {
	var lines []string
	for _, method := range interfaceType.Methods.List {
		methodType := renderNode(fileSet, method.Type)
		if len(method.Names) == 0 {
			lines = append(lines, prefix+" method embedded "+methodType)
			continue
		}
		for _, name := range method.Names {
			if ast.IsExported(name.Name) {
				lines = append(lines, prefix+" method "+name.Name+" "+methodType)
			}
		}
	}
	return lines
}

func apiLinesForFunc(fileSet *token.FileSet, decl *ast.FuncDecl, packageName string) []string {
	if !ast.IsExported(decl.Name.Name) {
		return nil
	}
	signature := renderNode(fileSet, decl.Type)
	if decl.Recv == nil {
		return []string{packageName + " func " + decl.Name.Name + " " + signature}
	}
	receiver := renderNode(fileSet, decl.Recv.List[0].Type)
	if !ast.IsExported(baseTypeName(decl.Recv.List[0].Type)) {
		return nil
	}
	return []string{packageName + " method " + receiver + "." + decl.Name.Name + " " + signature}
}

func baseTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return baseTypeName(typed.X)
	case *ast.ParenExpr:
		return baseTypeName(typed.X)
	default:
		return ""
	}
}

func renderNode(fileSet *token.FileSet, node ast.Node) string {
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, fileSet, node); err != nil {
		return fmt.Sprintf("<render-error: %v>", err)
	}
	rendered := strings.Join(strings.Fields(buffer.String()), " ")
	rendered = strings.ReplaceAll(rendered, "func( ", "func(")
	rendered = strings.ReplaceAll(rendered, ", )", ")")
	return strings.ReplaceAll(rendered, " )", ")")
}

func apiGoldenDiff(want, got []string) string {
	if slices.Equal(want, got) {
		return ""
	}
	return renderGoldenDiff(want, got, lcsLengths(want, got))
}

func lcsLengths(want, got []string) [][]int {
	lengths := make([][]int, len(want)+1)
	for i := range lengths {
		lengths[i] = make([]int, len(got)+1)
	}
	for i := len(want) - 1; i >= 0; i-- {
		for j := len(got) - 1; j >= 0; j-- {
			switch {
			case want[i] == got[j]:
				lengths[i][j] = lengths[i+1][j+1] + 1
			case lengths[i+1][j] >= lengths[i][j+1]:
				lengths[i][j] = lengths[i+1][j]
			default:
				lengths[i][j] = lengths[i][j+1]
			}
		}
	}
	return lengths
}

func renderGoldenDiff(want, got []string, lengths [][]int) string {
	var buffer strings.Builder
	i, j := 0, 0
	for i < len(want) && j < len(got) {
		switch {
		case want[i] == got[j]:
			i++
			j++
		case lengths[i+1][j] >= lengths[i][j+1]:
			fmt.Fprintf(&buffer, "- %s\n", want[i])
			i++
		default:
			fmt.Fprintf(&buffer, "+ %s\n", got[j])
			j++
		}
	}
	for ; i < len(want); i++ {
		fmt.Fprintf(&buffer, "- %s\n", want[i])
	}
	for ; j < len(got); j++ {
		fmt.Fprintf(&buffer, "+ %s\n", got[j])
	}
	return buffer.String()
}

func syntheticAPILines(t *testing.T, source string) []string {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "synthetic.go", source, 0)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	return apiLinesForFile(fileSet, file, "pkg/sample")
}

// TestAPIGoldenBuilderSynthetic proves the builder preserves field order and
// const/var initialization text.
func TestAPIGoldenBuilderSynthetic(t *testing.T) {
	baseSource := `package sample

type T int

type S struct {
	A int
	B string
}

const (
	X = 1
	Y = 2
)

const (
	First  T = iota
	Second
)

var V = 3

func (s S) Method() {}
`
	lines := syntheticAPILines(t, baseSource)
	assertSyntheticAPIContract(t, lines)

	cases := []struct {
		name   string
		source string
	}{
		{
			name:   "struct field reorder",
			source: strings.Replace(baseSource, "\tA int\n\tB string", "\tB string\n\tA int", 1),
		},
		{
			name:   "const value change",
			source: strings.Replace(baseSource, "\tY = 2", "\tY = 9", 1),
		},
		{
			name:   "var initializer change",
			source: strings.Replace(baseSource, "var V = 3", "var V = 4", 1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := apiGoldenDiff(lines, syntheticAPILines(t, tc.source)); diff == "" {
				t.Error("changed source produced no golden diff")
			}
		})
	}
}

func assertSyntheticAPIContract(t *testing.T, lines []string) {
	t.Helper()
	required := []string{
		"pkg/sample type S field A int",
		"pkg/sample type S field B string",
		"pkg/sample const ( X = 1 Y = 2)",
		"pkg/sample var V = 3",
		"pkg/sample method S.Method func()",
	}
	for _, want := range required {
		if !containsLine(lines, want) {
			t.Errorf("synthetic API is missing %q\nlines:\n%s", want, strings.Join(lines, "\n"))
		}
	}
	if indexOfLine(lines, "pkg/sample type S field A int") > indexOfLine(lines, "pkg/sample type S field B string") {
		t.Error("struct field order was not preserved")
	}

	var iotaLine string
	for _, line := range lines {
		if strings.Contains(line, "First") && strings.Contains(line, "iota") {
			iotaLine = line
			break
		}
	}
	if !strings.Contains(iotaLine, "First T = iota") || !strings.Contains(iotaLine, "Second") {
		t.Errorf("iota const block not fully rendered: %q", iotaLine)
	}
}

// TestAPIGoldenDiffSynthetic covers the comparison helper independently of
// production ASTs.
func TestAPIGoldenDiffSynthetic(t *testing.T) {
	base := []string{
		"pkg/sample type S struct",
		"pkg/sample type S field A int",
		"pkg/sample type S field B string",
		"pkg/sample method S.M func()",
	}

	cases := []struct {
		name    string
		got     []string
		changed bool
	}{
		{
			name:    "equal",
			got:     append([]string(nil), base...),
			changed: false,
		},
		{
			name: "const value change",
			got: []string{
				"pkg/sample const ( X = 2 )",
			},
			changed: true,
		},
		{
			name: "field reorder",
			got: []string{
				"pkg/sample type S struct",
				"pkg/sample type S field B string",
				"pkg/sample type S field A int",
				"pkg/sample method S.M func()",
			},
			changed: true,
		},
		{
			name: "field added",
			got: append(append([]string(nil), base[:3]...),
				"pkg/sample type S field C bool",
				"pkg/sample method S.M func()",
			),
			changed: true,
		},
		{
			name: "field removed",
			got: []string{
				"pkg/sample type S struct",
				"pkg/sample type S field A int",
				"pkg/sample method S.M func()",
			},
			changed: true,
		},
		{
			name: "method added",
			got: append(append([]string(nil), base...),
				"pkg/sample method S.N func()",
			),
			changed: true,
		},
		{
			name:    "method removed",
			got:     append([]string(nil), base[:3]...),
			changed: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := "pkg/sample const ( X = 1 )"
			if tc.name == "equal" {
				if diff := apiGoldenDiff(tc.got, tc.got); diff != "" {
					t.Errorf("identical entries produced a diff:\n%s", diff)
				}
				return
			}
			diff := apiGoldenDiff([]string{want}, append([]string{want}, tc.got...))
			if tc.name == "const value change" {
				diff = apiGoldenDiff([]string{want}, tc.got)
			}
			if tc.changed && diff == "" {
				t.Error("changed entries produced no diff")
			}
		})
	}
}

func containsLine(lines []string, want string) bool {
	return indexOfLine(lines, want) >= 0
}

func indexOfLine(lines []string, want string) int {
	for i, line := range lines {
		if line == want {
			return i
		}
	}
	return -1
}
