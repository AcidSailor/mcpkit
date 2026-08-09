// Package main stages GoReleaser output for mcpb pack.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// metadata contains the used fields from GoReleaser metadata.
type metadata struct {
	Version string `json:"version"`
}

// artifact contains the used fields from a GoReleaser artifact.
type artifact struct {
	Path   string `json:"path"`
	Goos   string `json:"goos"`
	Goarch string `json:"goarch"`
	Type   string `json:"type"`
}

// manifestServer mirrors the server.mcp_config block of an mcpb manifest.
type manifestServer struct {
	Server struct {
		MCPConfig struct {
			Command           string `json:"command"`
			PlatformOverrides map[string]struct {
				Command string `json:"command"`
			} `json:"platform_overrides"`
		} `json:"mcp_config"`
	} `json:"server"`
}

// binaryTarget identifies one platform binary in the bundle.
type binaryTarget struct {
	base   string // Bundle filename, optionally ending in .exe.
	name   string // GoReleaser binary name without OS and architecture.
	goos   string
	goarch string
}

// Stage assembles an mcpb directory from GoReleaser output and a manifest.
func Stage(distDir, manifestPath, outDir string) error {
	meta, err := readMetadata(filepath.Join(distDir, "metadata.json"))
	if err != nil {
		return err
	}

	manBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	targets, err := parseTargets(manBytes)
	if err != nil {
		return err
	}

	arts, err := readArtifacts(filepath.Join(distDir, "artifacts.json"))
	if err != nil {
		return err
	}

	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("clean out dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "server"), 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	// Copy manifest assets before replacing manifest.json with its stamped form.
	if err := copyTree(filepath.Dir(manifestPath), outDir); err != nil {
		return err
	}
	stamped, err := stampVersion(manBytes, meta.Version)
	if err != nil {
		return err
	}
	if err := os.WriteFile(
		filepath.Join(outDir, "manifest.json"),
		stamped,
		0o644,
	); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	for _, t := range targets {
		src, err := findArtifact(arts, t)
		if err != nil {
			return err
		}
		if err := copyFile(
			src,
			filepath.Join(outDir, "server", t.base),
			0o755,
		); err != nil {
			return fmt.Errorf("copy %s: %w", t.base, err)
		}
	}
	return nil
}

func readMetadata(p string) (metadata, error) {
	var m metadata
	b, err := os.ReadFile(p)
	if err != nil {
		return m, fmt.Errorf("read metadata: %w", err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("parse metadata: %w", err)
	}
	if m.Version == "" {
		return m, fmt.Errorf("metadata: empty version in %s", p)
	}
	return m, nil
}

func readArtifacts(p string) ([]artifact, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read artifacts: %w", err)
	}
	var arts []artifact
	if err := json.Unmarshal(b, &arts); err != nil {
		return nil, fmt.Errorf("parse artifacts: %w", err)
	}
	return arts, nil
}

// parseTargets returns the unique binaries declared by the manifest.
func parseTargets(manBytes []byte) ([]binaryTarget, error) {
	var ms manifestServer
	if err := json.Unmarshal(manBytes, &ms); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	// Keep labels so an empty command identifies its manifest field.
	cmds := []struct{ plat, cmd string }{
		{"mcp_config.command", ms.Server.MCPConfig.Command},
	}
	for plat, ov := range ms.Server.MCPConfig.PlatformOverrides {
		cmds = append(cmds, struct{ plat, cmd string }{
			"platform_overrides." + plat, ov.Command,
		})
	}

	var targets []binaryTarget
	seen := map[string]bool{}
	for _, c := range cmds {
		if c.cmd == "" {
			return nil, fmt.Errorf("manifest %s is empty", c.plat)
		}
		base := path.Base(c.cmd) // Remove the manifest directory prefix.
		if seen[base] {
			continue
		}
		seen[base] = true
		name, goos, goarch, err := splitTarget(base)
		if err != nil {
			return nil, err
		}
		targets = append(targets, binaryTarget{
			base: base, name: name, goos: goos, goarch: goarch,
		})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("manifest declares no server binaries")
	}
	return targets, nil
}

// splitTarget parses the final OS and architecture fields in a binary name.
func splitTarget(base string) (name, goos, goarch string, err error) {
	trimmed := strings.TrimSuffix(base, ".exe")
	parts := strings.Split(trimmed, "-")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf(
			"cannot parse os/arch from binary name %q",
			base,
		)
	}
	goos = parts[len(parts)-2]
	goarch = parts[len(parts)-1]
	name = strings.Join(parts[:len(parts)-2], "-")
	return name, goos, goarch, nil
}

// findArtifact resolves one exact binary artifact for a target.
func findArtifact(arts []artifact, t binaryTarget) (string, error) {
	var matches []artifact
	for _, a := range arts {
		if a.Type == "Binary" && a.Goos == t.goos && a.Goarch == t.goarch {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf(
			"no Binary artifact for %s/%s in artifacts.json",
			t.goos,
			t.goarch,
		)
	case 1:
		return matches[0].Path, nil
	}
	// Disambiguate multiple platform builds by binary name.
	var named []artifact
	for _, a := range matches {
		if binaryName(a.Path) == t.name {
			named = append(named, a)
		}
	}
	if len(named) == 1 {
		return named[0].Path, nil
	}
	paths := make([]string, len(matches))
	for i, a := range matches {
		paths[i] = a.Path
	}
	return "", fmt.Errorf(
		"ambiguous Binary artifacts for %s (%s/%s): %s",
		t.base,
		t.goos,
		t.goarch,
		strings.Join(paths, ", "),
	)
}

// binaryName returns the artifact base name without an .exe suffix.
func binaryName(p string) string {
	return strings.TrimSuffix(filepath.Base(p), ".exe")
}

// stampVersion updates version while preserving all other manifest fields.
func stampVersion(manBytes []byte, version string) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(manBytes, &doc); err != nil {
		return nil, fmt.Errorf("parse manifest for stamping: %w", err)
	}
	doc["version"] = version
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal stamped manifest: %w", err)
	}
	return append(out, '\n'), nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

// copyTree copies a flat asset directory and rejects nested directories.
func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read manifest dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			return fmt.Errorf(
				"unsupported nested directory %q in %s: bundle assets must be flat",
				e.Name(),
				src,
			)
		}
		if err := copyFile(
			filepath.Join(src, e.Name()),
			filepath.Join(dst, e.Name()),
			0o644,
		); err != nil {
			return err
		}
	}
	return nil
}
