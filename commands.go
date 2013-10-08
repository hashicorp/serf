package main

import (
	"github.com/hashicorp/serf/cli"
)

// Commands is the mapping of all the available Serf commands.
var Commands map[string]cli.CommandFactory

func init() {
	Commands = map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			return &cli.AgentCommand{}, nil
		},

		"version": func() (cli.Command, error) {
			return &cli.VersionCommand{
				Revision:          GitCommit,
				Version:           Version,
				VersionPrerelease: VersionPrerelease,
			}, nil
		},
	}
}
