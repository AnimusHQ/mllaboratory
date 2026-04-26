package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadHandlersDoNotMutateRunState(t *testing.T) {
	dir := testDir(t)
	specFile := filepath.Join(dir, "run_spec_api.go")
	planFile := filepath.Join(dir, "run_plan_api.go")
	dryRunFile := filepath.Join(dir, "run_dryrun_api.go")

	requireNoMutation(t, specFile, "handleGetRun", false)
	requireNoMutation(t, planFile, "handleGetRunPlan", false)
	requireNoMutation(t, dryRunFile, "handleGetDryRun", false)
}

func requireNoMutation(t *testing.T, path, funcName string, requireDerive bool) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name != nil && d.Name.Name == funcName {
			fn = d
			break
		}
	}
	if fn == nil {
		t.Fatalf("missing %s in %s", funcName, path)
	}

	forbidden := map[string]struct{}{
		"DeriveAndPersistWithAudit":  {},
		"MarkDryRunRunningWithAudit": {},
	}
	foundDerive := false

	ast.Inspect(fn, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if selector.Sel == nil {
			return true
		}
		name := selector.Sel.Name
		if _, blocked := forbidden[name]; blocked {
			t.Fatalf("%s should not call %s", funcName, name)
		}
		if name == "Derive" {
			foundDerive = true
		}
		return true
	})

	if requireDerive && !foundDerive {
		t.Fatalf("%s should call Derive", funcName)
	}
}

func testDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to resolve test file path")
	}
	return filepath.Dir(filename)
}
