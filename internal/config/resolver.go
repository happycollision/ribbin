package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrCyclicExtends is returned when a cycle is detected in extends references
var ErrCyclicExtends = errors.New("cyclic extends detected")

// Resolver resolves effective shim configurations by processing extends inheritance.
type Resolver struct {
	// cache stores loaded external config files by their absolute path
	cache map[string]*ProjectConfig
}

// NewResolver creates a new Resolver instance.
func NewResolver() *Resolver {
	return &Resolver{
		cache: make(map[string]*ProjectConfig),
	}
}

// ResolveEffectiveShims computes the effective shim map for a scope by resolving
// all extends references recursively and merging shims in order.
//
// Parameters:
//   - config: the ProjectConfig containing the scope
//   - configPath: absolute path to the config file (used for resolving relative extends)
//   - scope: the scope to resolve (nil means resolve root shims only)
//
// Returns the merged shim map where later extends and own shims override earlier ones.
func (r *Resolver) ResolveEffectiveShims(
	config *ProjectConfig,
	configPath string,
	scope *ScopeConfig,
) (map[string]ShimConfig, error) {
	visited := make(map[string]bool)
	return r.resolveEffectiveShimsInternal(config, configPath, scope, visited)
}

// resolveEffectiveShimsInternal is the recursive implementation with cycle detection.
func (r *Resolver) resolveEffectiveShimsInternal(
	config *ProjectConfig,
	configPath string,
	scope *ScopeConfig,
	visited map[string]bool,
) (map[string]ShimConfig, error) {
	configDir := filepath.Dir(configPath)
	result := make(map[string]ShimConfig)

	// If no scope, return root shims directly
	if scope == nil {
		for name, shim := range config.Shims {
			result[name] = shim
		}
		return result, nil
	}

	// Process extends in order
	for _, extRef := range scope.Extends {
		ref, err := ParseExtendsRef(extRef, configDir)
		if err != nil {
			return nil, fmt.Errorf("invalid extends %q: %w", extRef, err)
		}

		var inherited map[string]ShimConfig
		if ref.IsLocal {
			inherited, err = r.resolveLocalRef(config, configPath, ref.Fragment, visited)
		} else {
			inherited, err = r.resolveExternalRef(ref, visited)
		}
		if err != nil {
			return nil, err
		}

		// Merge inherited shims (later overrides earlier)
		for name, shim := range inherited {
			result[name] = shim
		}
	}

	// Merge scope's own shims (overrides all extends)
	for name, shim := range scope.Shims {
		result[name] = shim
	}

	return result, nil
}

// resolveLocalRef resolves a local reference (root or root.scope-name).
func (r *Resolver) resolveLocalRef(
	config *ProjectConfig,
	configPath string,
	fragment string,
	visited map[string]bool,
) (map[string]ShimConfig, error) {
	// Create a key for cycle detection
	visitKey := configPath + "#" + fragment
	if visited[visitKey] {
		return nil, fmt.Errorf("%w: %s", ErrCyclicExtends, visitKey)
	}
	visited[visitKey] = true
	defer func() { visited[visitKey] = false }()

	if fragment == "root" {
		// Return root shims directly (no recursion needed for root)
		result := make(map[string]ShimConfig)
		for name, shim := range config.Shims {
			result[name] = shim
		}
		return result, nil
	}

	// fragment is "root.scope-name"
	scopeName := strings.TrimPrefix(fragment, "root.")
	targetScope, ok := config.Scopes[scopeName]
	if !ok {
		return nil, fmt.Errorf("scope %q not found in config", scopeName)
	}

	// Recursively resolve the target scope's extends
	return r.resolveEffectiveShimsInternal(config, configPath, &targetScope, visited)
}

// resolveExternalRef resolves an external file reference.
func (r *Resolver) resolveExternalRef(
	ref *ExtendsRef,
	visited map[string]bool,
) (map[string]ShimConfig, error) {
	// Load the external config (with caching)
	extConfig, err := r.loadExternalConfig(ref.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load external config %q: %w", ref.FilePath, err)
	}

	if ref.Fragment == "" {
		// No fragment means merge entire file: root shims + all scopes
		return r.resolveEntireFile(extConfig, ref.FilePath, visited)
	}

	// Fragment specified: resolve specific target
	return r.resolveLocalRef(extConfig, ref.FilePath, ref.Fragment, visited)
}

// resolveEntireFile merges an entire external config file (root + all scopes).
func (r *Resolver) resolveEntireFile(
	config *ProjectConfig,
	configPath string,
	visited map[string]bool,
) (map[string]ShimConfig, error) {
	result := make(map[string]ShimConfig)

	// Start with root shims
	for name, shim := range config.Shims {
		result[name] = shim
	}

	// Merge each scope's effective shims
	for _, scope := range config.Scopes {
		scopeCopy := scope
		scopeShims, err := r.resolveEffectiveShimsInternal(config, configPath, &scopeCopy, visited)
		if err != nil {
			return nil, err
		}
		for name, shim := range scopeShims {
			result[name] = shim
		}
	}

	return result, nil
}

// loadExternalConfig loads a config file, using the cache if available.
func (r *Resolver) loadExternalConfig(path string) (*ProjectConfig, error) {
	if config, ok := r.cache[path]; ok {
		return config, nil
	}

	config, err := LoadProjectConfig(path)
	if err != nil {
		return nil, err
	}

	r.cache[path] = config
	return config, nil
}
