package command

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/serf/cli"
	"github.com/hashicorp/serf/serf"
)

// VersionCommand is a Command implementation prints the version.
type VersionCommand struct {
	Revision          string
	Version           string
	VersionPrerelease string
}

func (c *VersionCommand) Help() string {
	return ""
}

func (c *VersionCommand) Run(_ []string, ui cli.Ui) int {
	var versionString bytes.Buffer
	fmt.Fprintf(&versionString, "Serf v%s", c.Version)
	if c.VersionPrerelease != "" {
		fmt.Fprintf(&versionString, ".%s", c.VersionPrerelease)

		if c.Revision != "" {
			fmt.Fprintf(&versionString, " (%s)", c.Revision)
		}
	}

	ui.Output(versionString.String())
	ui.Output(fmt.Sprintf("Agent Protocol: %d (Understands back to: %d)",
		serf.ProtocolVersionMax, serf.ProtocolVersionMin))
	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Serf version"
}
