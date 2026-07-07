package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const testManifest = `{
  "manifest_version": "0.3",
  "name": "foo",
  "version": "",
  "server": {
    "type": "binary",
    "entry_point": "server/foo-darwin-arm64",
    "mcp_config": {
      "command": "${__dirname}/server/foo-darwin-arm64",
      "platform_overrides": {
        "linux": { "command": "${__dirname}/server/foo-linux-amd64" },
        "win32": { "command": "${__dirname}/server/foo-windows-amd64.exe" }
      }
    }
  }
}`

// fixture lays out a dist/ + mcpb/ under a temp root with three platform
// binaries and a matching artifacts.json. Artifact paths are absolute so Stage
// can open them regardless of cwd.
func fixture(t *testing.T) (root, dist, manifest string) {
	t.Helper()
	root = t.TempDir()
	dist = filepath.Join(root, "dist")
	manifest = filepath.Join(root, "mcpb", "manifest.json")

	writeFile(t, filepath.Join(dist, "metadata.json"), `{"version":"1.2.3"}`)
	darwin := filepath.Join(dist, "foo_darwin_arm64_v8.0", "foo")
	linux := filepath.Join(dist, "foo_linux_amd64_v1", "foo")
	windows := filepath.Join(dist, "foo_windows_amd64_v1", "foo.exe")
	writeFile(t, darwin, "DARWIN")
	writeFile(t, linux, "LINUX")
	writeFile(t, windows, "WINDOWS")

	arts := `[
	  {"type":"Binary","goos":"darwin","goarch":"arm64","path":"` + filepath.ToSlash(darwin) + `"},
	  {"type":"Binary","goos":"linux","goarch":"amd64","path":"` + filepath.ToSlash(linux) + `"},
	  {"type":"Binary","goos":"windows","goarch":"amd64","path":"` + filepath.ToSlash(windows) + `"},
	  {"type":"Archive","goos":"linux","goarch":"amd64","path":"ignored.tar.gz"}
	]`
	writeFile(t, filepath.Join(dist, "artifacts.json"), arts)
	writeFile(t, manifest, testManifest)
	// A flat sibling asset next to manifest.json: copyTree must carry it into
	// the bundle, and it must survive the stamped-manifest overwrite.
	writeFile(t, filepath.Join(root, "mcpb", "icon.png"), "ICON")
	return root, dist, manifest
}

func TestStage_LaysOutBundle(t *testing.T) {
	root, dist, manifest := fixture(t)
	out := filepath.Join(root, "dist", "mcpb")

	if err := Stage(dist, manifest, out); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man map[string]any
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if man["version"] != "1.2.3" {
		t.Errorf("version = %v, want 1.2.3", man["version"])
	}
	if man["name"] != "foo" {
		t.Errorf("name not preserved: %v", man["name"])
	}
	// Stamping must preserve nested keys, not just top-level scalars.
	cmd, _ := man["server"].(map[string]any)
	mcpCfg, _ := cmd["mcp_config"].(map[string]any)
	if mcpCfg["command"] != "${__dirname}/server/foo-darwin-arm64" {
		t.Errorf("nested command not preserved: %v", mcpCfg["command"])
	}
	if _, ok := mcpCfg["platform_overrides"].(map[string]any); !ok {
		t.Errorf("platform_overrides not preserved: %v", mcpCfg)
	}

	// The flat sibling asset must be copied verbatim.
	if b, err := os.ReadFile(filepath.Join(out, "icon.png")); err != nil {
		t.Errorf("icon.png not staged: %v", err)
	} else if string(b) != "ICON" {
		t.Errorf("icon.png content = %q, want ICON", b)
	}

	cases := map[string]string{
		"foo-darwin-arm64":      "DARWIN",
		"foo-linux-amd64":       "LINUX",
		"foo-windows-amd64.exe": "WINDOWS",
	}
	for name, want := range cases {
		p := filepath.Join(out, "server", name)
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s content = %q, want %q", name, got, want)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o755 {
				t.Errorf("%s mode = %v, want 0755", name, info.Mode().Perm())
			}
		}
	}
}

func TestStage_MissingBinaryFails(t *testing.T) {
	root, dist, manifest := fixture(t)
	darwin := filepath.Join(dist, "foo_darwin_arm64_v8.0", "foo")
	writeFile(
		t,
		filepath.Join(dist, "artifacts.json"),
		`[{"type":"Binary","goos":"darwin","goarch":"arm64","path":"`+filepath.ToSlash(
			darwin,
		)+`"}]`,
	)
	if err := Stage(dist, manifest, filepath.Join(root, "out")); err == nil {
		t.Fatal("expected error for missing linux/amd64 binary")
	}
}

func TestStage_MissingMetadataFails(t *testing.T) {
	root, dist, manifest := fixture(t)
	if err := os.Remove(filepath.Join(dist, "metadata.json")); err != nil {
		t.Fatal(err)
	}
	if err := Stage(dist, manifest, filepath.Join(root, "out")); err == nil {
		t.Fatal("expected error for missing metadata.json")
	}
}

// TestStage_IsIdempotent runs Stage twice into the same out dir (exercising the
// os.RemoveAll clean) and asserts the second run produces the same layout.
func TestStage_IsIdempotent(t *testing.T) {
	root, dist, manifest := fixture(t)
	out := filepath.Join(root, "dist", "mcpb")
	for i := range 2 {
		if err := Stage(dist, manifest, out); err != nil {
			t.Fatalf("Stage run %d: %v", i, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(out, "server", "foo-linux-amd64"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "LINUX" {
		t.Errorf("linux binary content = %q, want LINUX", got)
	}
}

// TestStage_NestedAssetDirFails asserts an unsupported nested asset directory
// fails loudly instead of being silently dropped from the bundle.
func TestStage_NestedAssetDirFails(t *testing.T) {
	root, dist, manifest := fixture(t)
	writeFile(t, filepath.Join(root, "mcpb", "assets", "logo.png"), "LOGO")
	if err := Stage(dist, manifest, filepath.Join(root, "out")); err == nil {
		t.Fatal("expected error for nested asset directory")
	}
}

// TestStage_EmptyOverrideCommandFails asserts a present-but-empty override
// command is rejected, naming the platform, rather than silently dropped.
func TestStage_EmptyOverrideCommandFails(t *testing.T) {
	root, dist, manifest := fixture(t)
	writeFile(t, manifest, `{
	  "name": "foo", "version": "",
	  "server": { "mcp_config": {
	    "command": "${__dirname}/server/foo-darwin-arm64",
	    "platform_overrides": { "linux": { "command": "" } }
	  } }
	}`)
	if err := Stage(dist, manifest, filepath.Join(root, "out")); err == nil {
		t.Fatal("expected error for empty override command")
	}
}

// TestStage_UnparseableCommandFails asserts a command whose basename lacks the
// <name>-<goos>-<goarch> shape is rejected (the splitTarget guard).
func TestStage_UnparseableCommandFails(t *testing.T) {
	root, dist, manifest := fixture(t)
	writeFile(t, manifest, `{
	  "name": "foo", "version": "",
	  "server": { "mcp_config": { "command": "${__dirname}/server/foo" } }
	}`)
	if err := Stage(dist, manifest, filepath.Join(root, "out")); err == nil {
		t.Fatal("expected error for unparseable command name")
	}
}

func TestFindArtifact(t *testing.T) {
	linuxV1 := artifact{
		Type:   "Binary",
		Goos:   "linux",
		Goarch: "amd64",
		Path:   "/d/foo_v1/foo",
	}
	linuxV3 := artifact{
		Type:   "Binary",
		Goos:   "linux",
		Goarch: "amd64",
		Path:   "/d/foo_v3/foo",
	}
	helper := artifact{
		Type:   "Binary",
		Goos:   "linux",
		Goarch: "amd64",
		Path:   "/d/helper/helper",
	}
	target := binaryTarget{
		base:   "foo-linux-amd64",
		name:   "foo",
		goos:   "linux",
		goarch: "amd64",
	}

	// Single match resolves.
	if p, err := findArtifact(
		[]artifact{linuxV1},
		target,
	); err != nil ||
		p != linuxV1.Path {
		t.Fatalf("single match: got %q, %v", p, err)
	}
	// Two same-name variants (goamd64 v1/v3) are irreducibly ambiguous → error.
	if _, err := findArtifact(
		[]artifact{linuxV1, linuxV3},
		target,
	); err == nil {
		t.Fatal("expected ambiguity error for goamd64 variants")
	}
	// Two builds for one platform: disambiguated by the manifest's binary name.
	if p, err := findArtifact(
		[]artifact{helper, linuxV1},
		target,
	); err != nil ||
		p != linuxV1.Path {
		t.Fatalf("multi-build disambiguation: got %q, %v", p, err)
	}
	// No match → error.
	if _, err := findArtifact(nil, target); err == nil {
		t.Fatal("expected error for no match")
	}
}
