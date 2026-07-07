package main

import "github.com/alecthomas/kong"

// cli defines the mcpbstage flags parsed by kong.
type cli struct {
	Dist     string `help:"GoReleaser output directory."             default:"dist"`
	Manifest string `help:"Path to the source mcpb manifest."        default:"mcpb/manifest.json"`
	Out      string `help:"Staging directory handed to 'mcpb pack'." default:"dist/mcpb"`
}

func main() {
	var c cli
	ctx := kong.Parse(
		&c,
		kong.Name("mcpbstage"),
		kong.Description(
			"Stage GoReleaser binaries + manifest into an .mcpb bundle directory.",
		),
	)
	ctx.FatalIfErrorf(Stage(c.Dist, c.Manifest, c.Out))
}
