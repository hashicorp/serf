package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"os"
	"strings"
)

type KeyringCommand struct {
	Ui cli.Ui
}

func (c *KeyringCommand) Help() string {
	helpText := `
Usage: serf keyring <command> [options]...

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -rpc-auth=""              RPC auth token of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *KeyringCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("keys", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	args = cmdFlags.Args()

	// Commands is a mapping of all available key commands
	var Commands map[string]cli.CommandFactory

	ui := &cli.BasicUi{Writer: os.Stdout}

	Commands = map[string]cli.CommandFactory{
		"install-key": func() (cli.Command, error) {
			return &InstallKeyCommand{
				Ui: ui,
			}, nil
		},

		"use-key": func() (cli.Command, error) {
			return &UseKeyCommand{
				Ui: ui,
			}, nil
		},

		"remove-key": func() (cli.Command, error) {
			return &RemoveKeyCommand{
				Ui: ui,
			}, nil
		},
	}

	cli := &cli.CLI{
		Args:     args,
		Commands: Commands,
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		return 1
	}

	return exitCode
}

func (c *KeyringCommand) Synopsis() string {
	return "Manage encryption keys used by Serf"
}
