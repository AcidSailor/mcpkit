// Package main implements mcpbstage, a build-time helper that assembles the
// staging directory an .mcpb bundle is packed from. It reads GoReleaser's dist/
// output plus the mcpb manifest, then lays out <out>/manifest.json (with the
// release version stamped in) and <out>/server/<binary> for every platform the
// manifest declares. The real `mcpb pack` still validates and zips the result.
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

// metadata mirrors the fields we need from GoReleaser's dist/metadata.json.
type metadata struct {
	Version string `json:"version"`
}

// artifact mirrors the fields we need from each dist/artifacts.json entry.
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

// binaryTarget is one platform binary the manifest asks the bundle to carry.
type binaryTarget struct {
	base   string // in-bundle filename, e.g. "foo-linux-amd64" (may end .exe)
	name   string // GoReleaser binary name, e.g. "foo" (base minus -goos-goarch)
	goos   string
	goarch string
}

// Stage assembles outDir from the GoReleaser output in distDir and the mcpb
// manifest at manifestPath. It is the whole tool; main() only parses flags.
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

	// Copy flat manifest siblings (e.g. icon.png), then overwrite with the
	// stamped manifest.json.
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

// parseTargets extracts the platform binaries the manifest declares from
// mcp_config.command and every platform_overrides.<plat>.command. A command
// present but empty is a malformed manifest and fails loudly, naming the
// platform, rather than silently dropping that platform from the bundle.
func parseTargets(manBytes []byte) ([]binaryTarget, error) {
	var ms manifestServer
	if err := json.Unmarshal(manBytes, &ms); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	// Keep the platform label alongside each command so an empty command can
	// name which declaration is at fault. The base command has no override key.
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
		base := path.Base(c.cmd) // last element: drops "${__dirname}/server/"
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

// splitTarget parses "<name>-<goos>-<goarch>[.exe]" into its name, goos and
// goarch by taking the last two dash-separated tokens (robust to dashes in
// <name>); everything before them is the GoReleaser binary name.
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

// findArtifact resolves the source binary for t. It matches on goos/goarch and,
// when a build emits several Binary artifacts for one platform (multiple
// GoReleaser builds, or goamd64/goarm variants), disambiguates by the binary
// name t asks for. It refuses to guess: zero or an irreducibly-ambiguous set of
// matches is an error, never a silently-wrong binary staged under t.base.
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
	// >1 match: narrow to the binary the manifest names (multiple builds).
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

// binaryName is the GoReleaser binary name for an artifact path: its last
// element with any ".exe" suffix removed, to compare against binaryTarget.name.
func binaryName(p string) string {
	return strings.TrimSuffix(filepath.Base(p), ".exe")
}

// stampVersion sets the top-level "version" field, preserving all other keys
// (the map round-trip reorders keys alphabetically — harmless, mcpb pack and
// the runtime read by key, not order).
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
	return os.Chmod(
		dst,
		mode,
	) // enforce mode even if a prior file/umask interfered
}

// copyTree copies the immediate files of src into dst. mcpb/ holds manifest.json
// plus optional flat assets, so non-recursive is sufficient; a nested directory
// is an unsupported layout and fails loudly rather than being silently dropped
// from the bundle.
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
