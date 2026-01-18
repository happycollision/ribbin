package security

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeReadlink_NotSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := SafeReadlink(file)
	if err == nil {
		t.Error("expected error for non-symlink, got nil")
	}
	if err != nil && !contains(err.Error(), "not a symlink") {
		t.Errorf("expected 'not a symlink' error, got: %v", err)
	}
}

func TestSafeReadlink_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")
	link := filepath.Join(tmpDir, "link")

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	resolved, err := SafeReadlink(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != target {
		t.Errorf("expected %s, got %s", target, resolved)
	}
}

func TestSafeReadlink_RelativeTarget(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")
	linkDir := filepath.Join(tmpDir, "subdir")
	link := filepath.Join(linkDir, "link")

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(linkDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create relative symlink
	if err := os.Symlink("../target", link); err != nil {
		t.Fatal(err)
	}

	resolved, err := SafeReadlink(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != target {
		t.Errorf("expected %s, got %s", target, resolved)
	}
}

func TestResolveSymlinkChain_Circular(t *testing.T) {
	tmpDir := t.TempDir()
	link1 := filepath.Join(tmpDir, "link1")
	link2 := filepath.Join(tmpDir, "link2")

	// Create circular symlinks
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveSymlinkChain(link1)
	if err == nil {
		t.Error("expected error for circular symlink, got nil")
	}
	if err != nil && !contains(err.Error(), "circular") {
		t.Errorf("expected 'circular' error, got: %v", err)
	}
}

func TestResolveSymlinkChain_TooDeep(t *testing.T) {
	tmpDir := t.TempDir()

	// Create chain longer than MaxSymlinkDepth
	prev := filepath.Join(tmpDir, "target")
	if err := os.WriteFile(prev, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= MaxSymlinkDepth+2; i++ {
		current := filepath.Join(tmpDir, fmt.Sprintf("link%d", i))
		if err := os.Symlink(prev, current); err != nil {
			t.Fatalf("failed to create symlink %d: %v", i, err)
		}
		prev = current
	}

	_, err := ResolveSymlinkChain(prev)
	if err == nil {
		t.Error("expected error for deep chain, got nil")
	}
	if err != nil && !contains(err.Error(), "too deep") {
		t.Errorf("expected 'too deep' error, got: %v", err)
	}
}

func TestResolveSymlinkChain_ValidChain(t *testing.T) {
	tmpDir := t.TempDir()

	// Create chain: link1 -> link2 -> target
	target := filepath.Join(tmpDir, "target")
	link2 := filepath.Join(tmpDir, "link2")
	link1 := filepath.Join(tmpDir, "link1")

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link2); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveSymlinkChain(link1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != target {
		t.Errorf("expected %s, got %s", target, resolved)
	}
}

func TestResolveSymlinkChain_NotSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveSymlinkChain(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != file {
		t.Errorf("expected %s, got %s", file, resolved)
	}
}

func TestValidateSymlinkTargetSafe_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	link := filepath.Join(tmpDir, "link")
	target := "../../etc/passwd"

	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	err := ValidateSymlinkTargetSafe(link, target)
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
	if err != nil && !contains(err.Error(), "traversal") {
		t.Errorf("expected 'traversal' error, got: %v", err)
	}
}

func TestValidateSymlinkTargetSafe_CriticalBinary(t *testing.T) {
	// Skip if not on a Unix-like system
	if _, err := os.Stat("/usr/bin/bash"); os.IsNotExist(err) {
		t.Skip("skipping: /usr/bin/bash not found")
	}

	tmpDir := t.TempDir()
	link := filepath.Join(tmpDir, "link")

	if err := os.Symlink("/usr/bin/bash", link); err != nil {
		t.Fatal(err)
	}

	err := ValidateSymlinkTargetSafe(link, "/usr/bin/bash")
	if err == nil {
		t.Error("expected error for critical binary, got nil")
	}
	if err != nil && !contains(err.Error(), "critical system binary") {
		t.Errorf("expected 'critical system binary' error, got: %v", err)
	}
}

func TestNoSymlinksInPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a symlink in the path
	linkDir := filepath.Join(tmpDir, "linkdir")
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetDir, linkDir); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(linkDir, "file")

	err := NoSymlinksInPath(file)
	if err == nil {
		t.Error("expected error for symlink in path, got nil")
	}
	if err != nil && !contains(err.Error(), "symlink in path") {
		t.Errorf("expected 'symlink in path' error, got: %v", err)
	}
}

func TestNoSymlinksInPath_NoSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	normalDir := filepath.Join(tmpDir, "normaldir")
	if err := os.Mkdir(normalDir, 0755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(normalDir, "file")

	err := NoSymlinksInPath(file)
	if err != nil {
		t.Errorf("unexpected error for path without symlinks: %v", err)
	}
}

func TestNoSymlinksInPath_NonexistentPath(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistent := filepath.Join(tmpDir, "does", "not", "exist")

	// Should not error on nonexistent paths (they might be created later)
	err := NoSymlinksInPath(nonexistent)
	if err != nil {
		t.Errorf("unexpected error for nonexistent path: %v", err)
	}
}

func TestGetSymlinkInfo_NotSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := GetSymlinkInfo(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsSymlink {
		t.Error("expected IsSymlink to be false")
	}
	if info.FinalTarget != file {
		t.Errorf("expected FinalTarget %s, got %s", file, info.FinalTarget)
	}
}

func TestGetSymlinkInfo_SingleLink(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")
	link := filepath.Join(tmpDir, "link")

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	info, err := GetSymlinkInfo(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsSymlink {
		t.Error("expected IsSymlink to be true")
	}
	if info.Target != target {
		t.Errorf("expected Target %s, got %s", target, info.Target)
	}
	if info.FinalTarget != target {
		t.Errorf("expected FinalTarget %s, got %s", target, info.FinalTarget)
	}
	if info.ChainDepth != 1 {
		t.Errorf("expected ChainDepth 1, got %d", info.ChainDepth)
	}
}

func TestGetSymlinkInfo_Chain(t *testing.T) {
	tmpDir := t.TempDir()

	// Create chain: link1 -> link2 -> target
	target := filepath.Join(tmpDir, "target")
	link2 := filepath.Join(tmpDir, "link2")
	link1 := filepath.Join(tmpDir, "link1")

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link2); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatal(err)
	}

	info, err := GetSymlinkInfo(link1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsSymlink {
		t.Error("expected IsSymlink to be true")
	}
	if info.Target != link2 {
		t.Errorf("expected Target %s, got %s", link2, info.Target)
	}
	if info.FinalTarget != target {
		t.Errorf("expected FinalTarget %s, got %s", target, info.FinalTarget)
	}
	if info.ChainDepth != 2 {
		t.Errorf("expected ChainDepth 2, got %d", info.ChainDepth)
	}
	if len(info.Chain) != 3 {
		t.Errorf("expected Chain length 3, got %d", len(info.Chain))
	}
}

func TestIsSymlinkSafe_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")
	link := filepath.Join(tmpDir, "link")

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	safe, err := IsSymlinkSafe(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !safe {
		t.Error("expected symlink to be safe")
	}
}

func TestIsSymlinkSafe_Circular(t *testing.T) {
	tmpDir := t.TempDir()
	link1 := filepath.Join(tmpDir, "link1")
	link2 := filepath.Join(tmpDir, "link2")

	if err := os.Symlink(link2, link1); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatal(err)
	}

	safe, err := IsSymlinkSafe(link1)
	if err == nil {
		t.Error("expected error for circular symlink")
	}
	if safe {
		t.Error("expected symlink to be unsafe")
	}
}

func TestValidateSymlinkForShimming_NotSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	target, err := ValidateSymlinkForShimming(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != file {
		t.Errorf("expected %s, got %s", file, target)
	}
}

func TestValidateSymlinkForShimming_ValidSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")
	link := filepath.Join(tmpDir, "link")

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	finalTarget, err := ValidateSymlinkForShimming(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if finalTarget != target {
		t.Errorf("expected %s, got %s", target, finalTarget)
	}
}

func TestValidateSymlinkForShimming_UnsafeSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	link := filepath.Join(tmpDir, "link1")
	link2 := filepath.Join(tmpDir, "link2")

	// Create circular symlinks
	if err := os.Symlink(link2, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link, link2); err != nil {
		t.Fatal(err)
	}

	_, err := ValidateSymlinkForShimming(link)
	if err == nil {
		t.Error("expected error for unsafe symlink")
	}
	if err != nil && !contains(err.Error(), "unsafe symlink") {
		t.Errorf("expected 'unsafe symlink' error, got: %v", err)
	}
}
