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
