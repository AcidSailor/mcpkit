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

	// Copy manifest siblings (icons/assets), then overwrite the stamped manifest.
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
		src, err := findArtifact(arts, t.goos, t.goarch)
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
// mcp_config.command and every platform_overrides.<plat>.command.
func parseTargets(manBytes []byte) ([]binaryTarget, error) {
	var ms manifestServer
	if err := json.Unmarshal(manBytes, &ms); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	cmds := []string{ms.Server.MCPConfig.Command}
	for _, ov := range ms.Server.MCPConfig.PlatformOverrides {
		cmds = append(cmds, ov.Command)
	}

	var targets []binaryTarget
	seen := map[string]bool{}
	for _, c := range cmds {
		if c == "" {
			continue
		}
		base := path.Base(c) // strip "${__dirname}/server/"
		if seen[base] {
			continue
		}
		seen[base] = true
		goos, goarch, err := splitTarget(base)
		if err != nil {
			return nil, err
		}
		targets = append(
			targets,
			binaryTarget{base: base, goos: goos, goarch: goarch},
		)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("manifest declares no server binaries")
	}
	return targets, nil
}

// splitTarget parses "<name>-<goos>-<goarch>[.exe]" into goos/goarch by taking
// the last two dash-separated tokens (robust to dashes in <name>).
func splitTarget(base string) (goos, goarch string, err error) {
	name := strings.TrimSuffix(base, ".exe")
	parts := strings.Split(name, "-")
	if len(parts) < 3 {
		return "", "", fmt.Errorf(
			"cannot parse os/arch from binary name %q",
			base,
		)
	}
	return parts[len(parts)-2], parts[len(parts)-1], nil
}

func findArtifact(arts []artifact, goos, goarch string) (string, error) {
	for _, a := range arts {
		if a.Type == "Binary" && a.Goos == goos && a.Goarch == goarch {
			return a.Path, nil
		}
	}
	return "", fmt.Errorf(
		"no Binary artifact for %s/%s in artifacts.json",
		goos,
		goarch,
	)
}

// stampVersion sets the top-level "version" field, preserving all other keys.
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
// plus optional flat assets, so non-recursive is sufficient.
func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read manifest dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
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
