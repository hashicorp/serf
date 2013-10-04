package cli

import (
	"bytes"
	"fmt"
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

func (c *VersionCommand) Run(_ []string, ui Ui) int {
	var versionString bytes.Buffer
	fmt.Fprintf(&versionString, "Serf v%s", c.Version)
	if c.VersionPrerelease != "" {
		fmt.Fprintf(&versionString, ".%s", c.VersionPrerelease)

		if c.Revision != "" {
			fmt.Fprintf(&versionString, " (%s)", c.Revision)
		}
	}

	ui.Output(versionString.String())
	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "prints the Serf version"
}
