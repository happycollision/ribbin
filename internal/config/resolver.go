package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrCyclicExtends is returned when a cycle is detected in extends references
var ErrCyclicExtends = errors.New("cyclic extends detected")

// ShimSource tracks where a shim configuration came from.
type ShimSource struct {
	// FilePath is the absolute path to the config file containing this shim
	FilePath string
	// Fragment identifies the location within the file: "root" or "root.scope-name"
	Fragment string
	// Overrode contains the source that this shim overrode, if any
	Overrode *ShimSource
}

// ResolvedShim wraps a ShimConfig with provenance information.
type ResolvedShim struct {
	// Config is the effective shim configuration
	Config ShimConfig
	// Source tracks where this configuration came from
	Source ShimSource
}

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

// MatchedScope represents a scope that matched the current working directory.
type MatchedScope struct {
	// Name is the scope name (key in config.Scopes)
	Name string
	// Config is the scope configuration
	Config ScopeConfig
}

// FindMatchingScope finds the most specific scope that matches the given working directory.
// Returns nil if no scope matches (root shims should be used).
// The configDir is the directory containing the config file.
func FindMatchingScope(config *ProjectConfig, configDir string, cwd string) *MatchedScope {
	var bestMatch *MatchedScope
	var bestMatchLen int

	for name, scope := range config.Scopes {
		// Determine the scope's absolute path
		scopePath := scope.Path
		if scopePath == "" || scopePath == "." {
			scopePath = configDir
		} else if !filepath.IsAbs(scopePath) {
			scopePath = filepath.Join(configDir, scopePath)
		}

		// Clean both paths for comparison
		scopePath = filepath.Clean(scopePath)
		cleanCwd := filepath.Clean(cwd)

		// Check if cwd is within or equal to the scope path
		if cleanCwd == scopePath || strings.HasPrefix(cleanCwd, scopePath+string(filepath.Separator)) {
			// This scope matches; check if it's more specific than the current best
			if len(scopePath) > bestMatchLen {
				bestMatchLen = len(scopePath)
				scopeCopy := scope
				bestMatch = &MatchedScope{
					Name:   name,
					Config: scopeCopy,
				}
			}
		}
	}

	return bestMatch
}

// ResolveEffectiveShimsWithProvenance computes the effective shim map with provenance tracking.
// It returns a map of command names to ResolvedShim structs that include source information.
//
// Parameters:
//   - config: the ProjectConfig containing the scope
//   - configPath: absolute path to the config file (used for resolving relative extends)
//   - scope: the scope to resolve (nil means resolve root shims only)
//   - scopeName: the name of the scope (used for fragment tracking), empty for root
//
// Returns the merged shim map with provenance where later extends and own shims override earlier ones.
func (r *Resolver) ResolveEffectiveShimsWithProvenance(
	config *ProjectConfig,
	configPath string,
	scope *ScopeConfig,
	scopeName string,
) (map[string]ResolvedShim, error) {
	visited := make(map[string]bool)
	return r.resolveWithProvenanceInternal(config, configPath, scope, scopeName, visited)
}

// resolveWithProvenanceInternal is the recursive implementation with cycle detection and provenance tracking.
func (r *Resolver) resolveWithProvenanceInternal(
	config *ProjectConfig,
	configPath string,
	scope *ScopeConfig,
	scopeName string,
	visited map[string]bool,
) (map[string]ResolvedShim, error) {
	configDir := filepath.Dir(configPath)
	result := make(map[string]ResolvedShim)

	// Determine the fragment for provenance tracking
	fragment := "root"
	if scopeName != "" {
		fragment = "root." + scopeName
	}

	// If no scope, return root shims directly with provenance
	if scope == nil {
		for name, shim := range config.Shims {
			result[name] = ResolvedShim{
				Config: shim,
				Source: ShimSource{
					FilePath: configPath,
					Fragment: "root",
				},
			}
		}
		return result, nil
	}

	// Process extends in order
	for _, extRef := range scope.Extends {
		ref, err := ParseExtendsRef(extRef, configDir)
		if err != nil {
			return nil, fmt.Errorf("invalid extends %q: %w", extRef, err)
		}

		var inherited map[string]ResolvedShim
		if ref.IsLocal {
			inherited, err = r.resolveLocalRefWithProvenance(config, configPath, ref.Fragment, visited)
		} else {
			inherited, err = r.resolveExternalRefWithProvenance(ref, visited)
		}
		if err != nil {
			return nil, err
		}

		// Merge inherited shims (later overrides earlier, tracking what was overridden)
		for name, resolved := range inherited {
			if existing, ok := result[name]; ok {
				// Track what we're overriding
				existingSource := existing.Source
				resolved.Source.Overrode = &existingSource
			}
			result[name] = resolved
		}
	}

	// Merge scope's own shims (overrides all extends)
	for name, shim := range scope.Shims {
		newResolved := ResolvedShim{
			Config: shim,
			Source: ShimSource{
				FilePath: configPath,
				Fragment: fragment,
			},
		}
		if existing, ok := result[name]; ok {
			existingSource := existing.Source
			newResolved.Source.Overrode = &existingSource
		}
		result[name] = newResolved
	}

	return result, nil
}

// resolveLocalRefWithProvenance resolves a local reference with provenance tracking.
func (r *Resolver) resolveLocalRefWithProvenance(
	config *ProjectConfig,
	configPath string,
	fragment string,
	visited map[string]bool,
) (map[string]ResolvedShim, error) {
	// Create a key for cycle detection
	visitKey := configPath + "#" + fragment
	if visited[visitKey] {
		return nil, fmt.Errorf("%w: %s", ErrCyclicExtends, visitKey)
	}
	visited[visitKey] = true
	defer func() { visited[visitKey] = false }()

	if fragment == "root" {
		// Return root shims directly with provenance
		result := make(map[string]ResolvedShim)
		for name, shim := range config.Shims {
			result[name] = ResolvedShim{
				Config: shim,
				Source: ShimSource{
					FilePath: configPath,
					Fragment: "root",
				},
			}
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
	return r.resolveWithProvenanceInternal(config, configPath, &targetScope, scopeName, visited)
}

// resolveExternalRefWithProvenance resolves an external file reference with provenance tracking.
func (r *Resolver) resolveExternalRefWithProvenance(
	ref *ExtendsRef,
	visited map[string]bool,
) (map[string]ResolvedShim, error) {
	// Load the external config (with caching)
	extConfig, err := r.loadExternalConfig(ref.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load external config %q: %w", ref.FilePath, err)
	}

	if ref.Fragment == "" {
		// No fragment means merge entire file: root shims + all scopes
		return r.resolveEntireFileWithProvenance(extConfig, ref.FilePath, visited)
	}

	// Fragment specified: resolve specific target
	return r.resolveLocalRefWithProvenance(extConfig, ref.FilePath, ref.Fragment, visited)
}

// resolveEntireFileWithProvenance merges an entire external config file with provenance tracking.
func (r *Resolver) resolveEntireFileWithProvenance(
	config *ProjectConfig,
	configPath string,
	visited map[string]bool,
) (map[string]ResolvedShim, error) {
	result := make(map[string]ResolvedShim)

	// Start with root shims
	for name, shim := range config.Shims {
		result[name] = ResolvedShim{
			Config: shim,
			Source: ShimSource{
				FilePath: configPath,
				Fragment: "root",
			},
		}
	}

	// Merge each scope's effective shims
	for scopeName, scope := range config.Scopes {
		scopeCopy := scope
		scopeShims, err := r.resolveWithProvenanceInternal(config, configPath, &scopeCopy, scopeName, visited)
		if err != nil {
			return nil, err
		}
		for name, resolved := range scopeShims {
			if existing, ok := result[name]; ok {
				existingSource := existing.Source
				resolved.Source.Overrode = &existingSource
			}
			result[name] = resolved
		}
	}

	return result, nil
}

// GetEffectiveConfigForCwd returns the effective shim configuration for the current working directory.
// It finds the nearest config file, determines the matching scope, and resolves all shims with provenance.
func GetEffectiveConfigForCwd() (configPath string, matchedScope *MatchedScope, shims map[string]ResolvedShim, err error) {
	// Find the config file
	configPath, err = FindProjectConfig()
	if err != nil {
		return "", nil, nil, err
	}
	if configPath == "" {
		return "", nil, nil, nil // No config found, not an error
	}

	// Load the config
	config, err := LoadProjectConfig(configPath)
	if err != nil {
		return configPath, nil, nil, err
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return configPath, nil, nil, err
	}

	// Find matching scope
	configDir := filepath.Dir(configPath)
	matchedScope = FindMatchingScope(config, configDir, cwd)

	// Resolve effective shims with provenance
	resolver := NewResolver()
	var scope *ScopeConfig
	var scopeName string
	if matchedScope != nil {
		scope = &matchedScope.Config
		scopeName = matchedScope.Name
	}

	shims, err = resolver.ResolveEffectiveShimsWithProvenance(config, configPath, scope, scopeName)
	if err != nil {
		return configPath, matchedScope, nil, err
	}

	return configPath, matchedScope, shims, nil
}
