package command

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/hashicorp/serf/cli"
	"strings"
)

// KeygenCommand is a Command implementation prints the version.
type KeygenCommand struct{}

func (c *KeygenCommand) Run(_ []string, ui cli.Ui) int {
	key := make([]byte, 16)
	n, err := rand.Reader.Read(key)
	if err != nil {
		ui.Error(fmt.Sprintf("Error reading random data: %s", err))
		return 1
	}
	if n != 16 {
		ui.Error(fmt.Sprintf("Couldn't read enough entropy. Generate more entropy!"))
		return 1
	}

	ui.Output(base64.StdEncoding.EncodeToString(key))
	return 0
}

func (c *KeygenCommand) Synopsis() string {
	return "Generates a new encryption key"
}

func (c *KeygenCommand) Help() string {
	helpText := `
Usage: serf keygen

  Generates a new encryption key that can be used to configure the
  agent to encrypt traffic. The output of this command is already
  in the proper format that the agent expects.
`
	return strings.TrimSpace(helpText)
}
