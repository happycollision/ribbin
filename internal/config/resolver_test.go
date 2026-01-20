package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/happycollision/ribbin/internal/testsafety"
)

func TestResolveEffectiveShims_IsolatedScope(t *testing.T) {
	// Scope with no extends should only have its own shims
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat": {Action: "block", Message: "root cat"},
		},
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Path: "apps/frontend",
				// No extends - isolated
				Wrappers: map[string]ShimConfig{
					"npm": {Action: "block", Message: "use pnpm"},
				},
			},
		},
	}

	scope := config.Scopes["frontend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShims(config, "/project/ribbin.jsonc", &scope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error = %v", err)
	}

	// Should only have npm, not cat (isolated scope)
	if len(result) != 1 {
		t.Errorf("expected 1 shim, got %d: %v", len(result), result)
	}
	if _, ok := result["npm"]; !ok {
		t.Error("expected npm shim")
	}
	if _, ok := result["cat"]; ok {
		t.Error("should not have cat shim (isolated scope)")
	}
}

func TestResolveEffectiveShims_ExtendsRoot(t *testing.T) {
	// Scope extends root - gets root shims + own shims, own wins on conflict
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat":  {Action: "block", Message: "root cat"},
			"grep": {Action: "warn", Message: "root grep"},
		},
		Scopes: map[string]ScopeConfig{
			"backend": {
				Path:    "apps/backend",
				Extends: []string{"root"},
				Wrappers: map[string]ShimConfig{
					"cat":  {Action: "redirect", Message: "backend cat"}, // overrides root
					"yarn": {Action: "block", Message: "use npm"},
				},
			},
		},
	}

	scope := config.Scopes["backend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShims(config, "/project/ribbin.jsonc", &scope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error = %v", err)
	}

	// Should have: cat (overridden), grep (from root), yarn (own)
	if len(result) != 3 {
		t.Errorf("expected 3 shims, got %d: %v", len(result), result)
	}

	// cat should be overridden by scope
	if result["cat"].Message != "backend cat" {
		t.Errorf("cat should be overridden, got %q", result["cat"].Message)
	}

	// grep should come from root
	if result["grep"].Message != "root grep" {
		t.Errorf("grep should come from root, got %q", result["grep"].Message)
	}

	// yarn is scope's own
	if result["yarn"].Message != "use npm" {
		t.Errorf("yarn should be from scope, got %q", result["yarn"].Message)
	}
}

func TestResolveEffectiveShims_MultipleExtends(t *testing.T) {
	// extends = ["root", "root.hardened"] - order matters, later wins
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat": {Action: "warn", Message: "root cat"},
			"rm":  {Action: "warn", Message: "root rm"},
		},
		Scopes: map[string]ScopeConfig{
			"hardened": {
				// No extends, just defines more restrictive rules
				Wrappers: map[string]ShimConfig{
					"cat": {Action: "block", Message: "hardened cat"}, // more restrictive than root
					"rm":  {Action: "block", Message: "hardened rm"},
				},
			},
			"backend": {
				Path:    "apps/backend",
				Extends: []string{"root", "root.hardened"},
				Wrappers: map[string]ShimConfig{
					"yarn": {Action: "block", Message: "use npm"},
				},
			},
		},
	}

	scope := config.Scopes["backend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShims(config, "/project/ribbin.jsonc", &scope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error = %v", err)
	}

	// cat and rm should come from hardened (later in extends list)
	if result["cat"].Message != "hardened cat" {
		t.Errorf("cat should be from hardened, got %q", result["cat"].Message)
	}
	if result["rm"].Message != "hardened rm" {
		t.Errorf("rm should be from hardened, got %q", result["rm"].Message)
	}
	if result["yarn"].Message != "use npm" {
		t.Errorf("yarn should be from scope, got %q", result["yarn"].Message)
	}
}

func TestResolveEffectiveShims_RecursiveExtends(t *testing.T) {
	// Scope A extends scope B which extends root
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat": {Action: "warn", Message: "root cat"},
		},
		Scopes: map[string]ScopeConfig{
			"base": {
				Extends: []string{"root"},
				Wrappers: map[string]ShimConfig{
					"npm": {Action: "block", Message: "base npm"},
				},
			},
			"frontend": {
				Path:    "apps/frontend",
				Extends: []string{"root.base"},
				Wrappers: map[string]ShimConfig{
					"yarn": {Action: "block", Message: "frontend yarn"},
				},
			},
		},
	}

	scope := config.Scopes["frontend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShims(config, "/project/ribbin.jsonc", &scope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error = %v", err)
	}

	// Should have: cat (from root via base), npm (from base), yarn (own)
	if len(result) != 3 {
		t.Errorf("expected 3 shims, got %d: %v", len(result), result)
	}
	if result["cat"].Message != "root cat" {
		t.Errorf("cat should come from root, got %q", result["cat"].Message)
	}
	if result["npm"].Message != "base npm" {
		t.Errorf("npm should come from base, got %q", result["npm"].Message)
	}
	if result["yarn"].Message != "frontend yarn" {
		t.Errorf("yarn should come from scope, got %q", result["yarn"].Message)
	}
}

func TestResolveEffectiveShims_CycleDetection(t *testing.T) {
	// A extends B, B extends A - should error
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"a": {
				Extends: []string{"root.b"},
				Wrappers:   map[string]ShimConfig{},
			},
			"b": {
				Extends: []string{"root.a"},
				Wrappers:   map[string]ShimConfig{},
			},
		},
	}

	scope := config.Scopes["a"]
	resolver := NewResolver()
	_, err := resolver.ResolveEffectiveShims(config, "/project/ribbin.jsonc", &scope)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cyclic") {
		t.Errorf("expected cyclic error, got: %v", err)
	}
}

func TestResolveEffectiveShims_SelfCycle(t *testing.T) {
	// Scope extends itself
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"self": {
				Extends: []string{"root.self"},
				Wrappers:   map[string]ShimConfig{},
			},
		},
	}

	scope := config.Scopes["self"]
	resolver := NewResolver()
	_, err := resolver.ResolveEffectiveShims(config, "/project/ribbin.jsonc", &scope)
	if err == nil {
		t.Fatal("expected cycle detection error for self-reference")
	}
	if !strings.Contains(err.Error(), "cyclic") {
		t.Errorf("expected cyclic error, got: %v", err)
	}
}

func TestResolveEffectiveShims_NilScope(t *testing.T) {
	// nil scope means resolve root shims only
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat":  {Action: "block", Message: "root cat"},
			"grep": {Action: "warn", Message: "root grep"},
		},
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Wrappers: map[string]ShimConfig{
					"npm": {Action: "block", Message: "use pnpm"},
				},
			},
		},
	}

	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShims(config, "/project/ribbin.jsonc", nil)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error = %v", err)
	}

	// Should only have root shims
	if len(result) != 2 {
		t.Errorf("expected 2 shims, got %d: %v", len(result), result)
	}
	if _, ok := result["cat"]; !ok {
		t.Error("expected cat shim from root")
	}
	if _, ok := result["npm"]; ok {
		t.Error("should not have npm shim (that's in a scope)")
	}
}

func TestResolveEffectiveShims_ScopeNotFound(t *testing.T) {
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Extends: []string{"root.nonexistent"},
				Wrappers:   map[string]ShimConfig{},
			},
		},
	}

	scope := config.Scopes["frontend"]
	resolver := NewResolver()
	_, err := resolver.ResolveEffectiveShims(config, "/project/ribbin.jsonc", &scope)
	if err == nil {
		t.Fatal("expected error for nonexistent scope")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestResolveEffectiveShims_ExternalFile(t *testing.T) {
	// Create a temporary external config file
	tmpDir := t.TempDir()

	// Create external config in a subdirectory (must be named ribbin.jsonc per security rules)
	externalDir := filepath.Join(tmpDir, "external")
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		t.Fatalf("failed to create external dir: %v", err)
	}
	externalPath := filepath.Join(externalDir, "ribbin.jsonc")
	externalContent := `{
  "wrappers": {
    "external-cmd": {
      "action": "block",
      "message": "from external"
    }
  }
}
`
	if err := os.WriteFile(externalPath, []byte(externalContent), 0644); err != nil {
		t.Fatalf("failed to write external config: %v", err)
	}

	// Create main config that extends external
	mainPath := filepath.Join(tmpDir, "ribbin.jsonc")
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat": {Action: "block", Message: "main cat"},
		},
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Path:    "apps/frontend",
				Extends: []string{"./external/ribbin.jsonc"},
				Wrappers: map[string]ShimConfig{
					"npm": {Action: "block", Message: "use pnpm"},
				},
			},
		},
	}

	scope := config.Scopes["frontend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShims(config, mainPath, &scope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error = %v", err)
	}

	// Should have: external-cmd (from external file), npm (own)
	// Note: cat is NOT included because the scope doesn't extend root
	if result["external-cmd"].Message != "from external" {
		t.Errorf("external-cmd should come from external file, got %q", result["external-cmd"].Message)
	}
	if result["npm"].Message != "use pnpm" {
		t.Errorf("npm should be from scope, got %q", result["npm"].Message)
	}
	if _, ok := result["cat"]; ok {
		t.Error("should not have cat (scope doesn't extend root)")
	}
}

func TestResolveEffectiveShims_ExternalFileWithFragment(t *testing.T) {
	// Create a temporary external config file with scopes
	tmpDir := t.TempDir()

	// Create external config with a scope (in subdirectory, named ribbin.jsonc)
	teamDir := filepath.Join(tmpDir, "team")
	if err := os.MkdirAll(teamDir, 0755); err != nil {
		t.Fatalf("failed to create team dir: %v", err)
	}
	externalPath := filepath.Join(teamDir, "ribbin.jsonc")
	externalContent := `{
  "wrappers": {
    "team-cmd": {
      "action": "warn",
      "message": "team root"
    }
  },
  "scopes": {
    "hardened": {
      "wrappers": {
        "team-cmd": {
          "action": "block",
          "message": "team hardened"
        }
      }
    }
  }
}
`
	if err := os.WriteFile(externalPath, []byte(externalContent), 0644); err != nil {
		t.Fatalf("failed to write external config: %v", err)
	}

	// Create main config that extends specific scope from external
	mainPath := filepath.Join(tmpDir, "ribbin.jsonc")
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Path:    "apps/frontend",
				Extends: []string{"./team/ribbin.jsonc#root.hardened"},
				Wrappers:   map[string]ShimConfig{},
			},
		},
	}

	scope := config.Scopes["frontend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShims(config, mainPath, &scope)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims error = %v", err)
	}

	// Should have team-cmd from the hardened scope (block, not warn)
	if result["team-cmd"].Action != "block" {
		t.Errorf("team-cmd should be block from hardened scope, got %q", result["team-cmd"].Action)
	}
	if result["team-cmd"].Message != "team hardened" {
		t.Errorf("team-cmd message should be from hardened, got %q", result["team-cmd"].Message)
	}
}

func TestResolver_ConfigCaching(t *testing.T) {
	// Verify that external configs are cached
	tmpDir := t.TempDir()

	// Create external config in subdirectory (must be named ribbin.jsonc)
	externalDir := filepath.Join(tmpDir, "external")
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		t.Fatalf("failed to create external dir: %v", err)
	}
	externalPath := filepath.Join(externalDir, "ribbin.jsonc")
	externalContent := `{
  "wrappers": {
    "ext": {
      "action": "block",
      "message": "external"
    }
  }
}
`
	if err := os.WriteFile(externalPath, []byte(externalContent), 0644); err != nil {
		t.Fatalf("failed to write external config: %v", err)
	}

	mainPath := filepath.Join(tmpDir, "ribbin.jsonc")
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"a": {
				Extends: []string{"./external/ribbin.jsonc"},
				Wrappers:   map[string]ShimConfig{},
			},
			"b": {
				Extends: []string{"./external/ribbin.jsonc"},
				Wrappers:   map[string]ShimConfig{},
			},
		},
	}

	resolver := NewResolver()

	// Resolve scope a
	scopeA := config.Scopes["a"]
	_, err := resolver.ResolveEffectiveShims(config, mainPath, &scopeA)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims for a error = %v", err)
	}

	// Check cache has the external file
	if len(resolver.cache) != 1 {
		t.Errorf("expected 1 cached config, got %d", len(resolver.cache))
	}

	// Resolve scope b - should reuse cache
	scopeB := config.Scopes["b"]
	_, err = resolver.ResolveEffectiveShims(config, mainPath, &scopeB)
	if err != nil {
		t.Fatalf("ResolveEffectiveShims for b error = %v", err)
	}

	// Cache size should still be 1
	if len(resolver.cache) != 1 {
		t.Errorf("expected 1 cached config after second resolve, got %d", len(resolver.cache))
	}
}

// Tests for provenance tracking

func TestResolveEffectiveShimsWithProvenance_RootOnly(t *testing.T) {
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat":  {Action: "block", Message: "root cat"},
			"grep": {Action: "warn", Message: "root grep"},
		},
	}

	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShimsWithProvenance(config, "/project/ribbin.jsonc", nil, "")
	if err != nil {
		t.Fatalf("ResolveEffectiveShimsWithProvenance error = %v", err)
	}

	// Check shim count
	if len(result) != 2 {
		t.Errorf("expected 2 shims, got %d", len(result))
	}

	// Check cat shim provenance
	catShim, ok := result["cat"]
	if !ok {
		t.Fatal("expected cat shim")
	}
	if catShim.Config.Action != "block" {
		t.Errorf("cat action = %q, want %q", catShim.Config.Action, "block")
	}
	if catShim.Source.FilePath != "/project/ribbin.jsonc" {
		t.Errorf("cat source file = %q, want %q", catShim.Source.FilePath, "/project/ribbin.jsonc")
	}
	if catShim.Source.Fragment != "root" {
		t.Errorf("cat source fragment = %q, want %q", catShim.Source.Fragment, "root")
	}
	if catShim.Source.Overrode != nil {
		t.Error("cat should not have overrode set")
	}
}

func TestResolveEffectiveShimsWithProvenance_ScopeExtendsRoot(t *testing.T) {
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat":  {Action: "block", Message: "root cat"},
			"grep": {Action: "warn", Message: "root grep"},
		},
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Path:    "apps/frontend",
				Extends: []string{"root"},
				Wrappers: map[string]ShimConfig{
					"cat":  {Action: "redirect", Message: "frontend cat"}, // overrides root
					"yarn": {Action: "block", Message: "use npm"},
				},
			},
		},
	}

	scope := config.Scopes["frontend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShimsWithProvenance(config, "/project/ribbin.jsonc", &scope, "frontend")
	if err != nil {
		t.Fatalf("ResolveEffectiveShimsWithProvenance error = %v", err)
	}

	// Check cat shim (overridden by scope)
	catShim, ok := result["cat"]
	if !ok {
		t.Fatal("expected cat shim")
	}
	if catShim.Config.Action != "redirect" {
		t.Errorf("cat action = %q, want %q", catShim.Config.Action, "redirect")
	}
	if catShim.Source.Fragment != "root.frontend" {
		t.Errorf("cat source fragment = %q, want %q", catShim.Source.Fragment, "root.frontend")
	}
	// Should track that it overrode root
	if catShim.Source.Overrode == nil {
		t.Fatal("cat should have overrode set")
	}
	if catShim.Source.Overrode.Fragment != "root" {
		t.Errorf("cat overrode fragment = %q, want %q", catShim.Source.Overrode.Fragment, "root")
	}

	// Check grep shim (inherited from root)
	grepShim, ok := result["grep"]
	if !ok {
		t.Fatal("expected grep shim")
	}
	if grepShim.Source.Fragment != "root" {
		t.Errorf("grep source fragment = %q, want %q", grepShim.Source.Fragment, "root")
	}
	if grepShim.Source.Overrode != nil {
		t.Error("grep should not have overrode set")
	}

	// Check yarn shim (scope's own, no inheritance)
	yarnShim, ok := result["yarn"]
	if !ok {
		t.Fatal("expected yarn shim")
	}
	if yarnShim.Source.Fragment != "root.frontend" {
		t.Errorf("yarn source fragment = %q, want %q", yarnShim.Source.Fragment, "root.frontend")
	}
	if yarnShim.Source.Overrode != nil {
		t.Error("yarn should not have overrode set")
	}
}

func TestResolveEffectiveShimsWithProvenance_MultipleExtends(t *testing.T) {
	// extends = ["root", "root.hardened"] - later override earlier
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat": {Action: "warn", Message: "root cat"},
		},
		Scopes: map[string]ScopeConfig{
			"hardened": {
				Wrappers: map[string]ShimConfig{
					"cat": {Action: "block", Message: "hardened cat"},
				},
			},
			"backend": {
				Path:    "apps/backend",
				Extends: []string{"root", "root.hardened"},
				Wrappers:   map[string]ShimConfig{},
			},
		},
	}

	scope := config.Scopes["backend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShimsWithProvenance(config, "/project/ribbin.jsonc", &scope, "backend")
	if err != nil {
		t.Fatalf("ResolveEffectiveShimsWithProvenance error = %v", err)
	}

	// cat should come from hardened, which overrode root
	catShim, ok := result["cat"]
	if !ok {
		t.Fatal("expected cat shim")
	}
	if catShim.Config.Action != "block" {
		t.Errorf("cat action = %q, want %q", catShim.Config.Action, "block")
	}
	if catShim.Source.Fragment != "root.hardened" {
		t.Errorf("cat source fragment = %q, want %q", catShim.Source.Fragment, "root.hardened")
	}
	// Should track the override chain
	if catShim.Source.Overrode == nil {
		t.Fatal("cat should have overrode set")
	}
	if catShim.Source.Overrode.Fragment != "root" {
		t.Errorf("cat overrode fragment = %q, want %q", catShim.Source.Overrode.Fragment, "root")
	}
}

func TestResolveEffectiveShimsWithProvenance_ExternalFile(t *testing.T) {
	// Create a temporary external config file
	tmpDir := t.TempDir()

	externalDir := filepath.Join(tmpDir, "external")
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		t.Fatalf("failed to create external dir: %v", err)
	}
	externalPath := filepath.Join(externalDir, "ribbin.jsonc")
	externalContent := `{
  "wrappers": {
    "external-cmd": {
      "action": "block",
      "message": "from external"
    }
  }
}
`
	if err := os.WriteFile(externalPath, []byte(externalContent), 0644); err != nil {
		t.Fatalf("failed to write external config: %v", err)
	}

	mainPath := filepath.Join(tmpDir, "ribbin.jsonc")
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Path:    "apps/frontend",
				Extends: []string{"./external/ribbin.jsonc"},
				Wrappers: map[string]ShimConfig{
					"npm": {Action: "block", Message: "use pnpm"},
				},
			},
		},
	}

	scope := config.Scopes["frontend"]
	resolver := NewResolver()
	result, err := resolver.ResolveEffectiveShimsWithProvenance(config, mainPath, &scope, "frontend")
	if err != nil {
		t.Fatalf("ResolveEffectiveShimsWithProvenance error = %v", err)
	}

	// Check external-cmd provenance
	extShim, ok := result["external-cmd"]
	if !ok {
		t.Fatal("expected external-cmd shim")
	}
	if extShim.Source.FilePath != externalPath {
		t.Errorf("external-cmd source file = %q, want %q", extShim.Source.FilePath, externalPath)
	}
	if extShim.Source.Fragment != "root" {
		t.Errorf("external-cmd source fragment = %q, want %q", extShim.Source.Fragment, "root")
	}

	// Check npm provenance
	npmShim, ok := result["npm"]
	if !ok {
		t.Fatal("expected npm shim")
	}
	if npmShim.Source.FilePath != mainPath {
		t.Errorf("npm source file = %q, want %q", npmShim.Source.FilePath, mainPath)
	}
	if npmShim.Source.Fragment != "root.frontend" {
		t.Errorf("npm source fragment = %q, want %q", npmShim.Source.Fragment, "root.frontend")
	}
}

// Tests for FindMatchingScope

func TestFindMatchingScope_NoScopes(t *testing.T) {
	config := &ProjectConfig{
		Wrappers: map[string]ShimConfig{
			"cat": {Action: "block"},
		},
	}

	match := FindMatchingScope(config, "/project", "/project/src")
	if match != nil {
		t.Errorf("expected nil match, got scope %q", match.Name)
	}
}

func TestFindMatchingScope_ExactMatch(t *testing.T) {
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Path: "apps/frontend",
			},
		},
	}

	match := FindMatchingScope(config, "/project", "/project/apps/frontend")
	if match == nil {
		t.Fatal("expected a match")
	}
	if match.Name != "frontend" {
		t.Errorf("match name = %q, want %q", match.Name, "frontend")
	}
}

func TestFindMatchingScope_SubdirectoryMatch(t *testing.T) {
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Path: "apps/frontend",
			},
		},
	}

	match := FindMatchingScope(config, "/project", "/project/apps/frontend/src/components")
	if match == nil {
		t.Fatal("expected a match")
	}
	if match.Name != "frontend" {
		t.Errorf("match name = %q, want %q", match.Name, "frontend")
	}
}

func TestFindMatchingScope_MostSpecificWins(t *testing.T) {
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"apps": {
				Path: "apps",
			},
			"frontend": {
				Path: "apps/frontend",
			},
		},
	}

	// Should match the more specific scope
	match := FindMatchingScope(config, "/project", "/project/apps/frontend/src")
	if match == nil {
		t.Fatal("expected a match")
	}
	if match.Name != "frontend" {
		t.Errorf("match name = %q, want %q", match.Name, "frontend")
	}

	// Should match apps for backend
	match = FindMatchingScope(config, "/project", "/project/apps/backend")
	if match == nil {
		t.Fatal("expected a match")
	}
	if match.Name != "apps" {
		t.Errorf("match name = %q, want %q", match.Name, "apps")
	}
}

func TestFindMatchingScope_NoMatch(t *testing.T) {
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"frontend": {
				Path: "apps/frontend",
			},
		},
	}

	// Outside the scope path
	match := FindMatchingScope(config, "/project", "/project/lib/utils")
	if match != nil {
		t.Errorf("expected no match, got scope %q", match.Name)
	}
}

func TestFindMatchingScope_EmptyPath(t *testing.T) {
	config := &ProjectConfig{
		Scopes: map[string]ScopeConfig{
			"global": {
				Path: "", // defaults to "."
			},
		},
	}

	// Empty path means the scope applies to the config directory
	match := FindMatchingScope(config, "/project", "/project/anything/here")
	if match == nil {
		t.Fatal("expected a match")
	}
	if match.Name != "global" {
		t.Errorf("match name = %q, want %q", match.Name, "global")
	}
}
