// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
)

// ForceLeaveCommand is a Command implementation that tells a running Serf
// to force a member to enter the "left" state.
type ForceLeaveCommand struct {
	Ui cli.Ui
}

var _ cli.Command = &ForceLeaveCommand{}

func (c *ForceLeaveCommand) Run(args []string) int {
	var prune bool

	cmdFlags := flag.NewFlagSet("join", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&prune, "prune", false, "Remove agent forcibly from list of members")
	rpcAddr := RPCAddrFlag(cmdFlags)
	rpcAuth := RPCAuthFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	nodes := cmdFlags.Args()
	if len(nodes) != 1 {
		c.Ui.Error("A node name must be specified to force leave.")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	client, err := RPCClient(*rpcAddr, *rpcAuth)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	if prune {
		err = client.ForceLeavePrune(nodes[0])
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error force leaving: %s", err))
			return 1
		}
		return 0

	}

	err = client.ForceLeave(nodes[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error force leaving: %s", err))
		return 1
	}

	return 0
}

func (c *ForceLeaveCommand) Synopsis() string {
	return "Forces a member of the cluster to enter the \"left\" state"
}

func (c *ForceLeaveCommand) Help() string {
	helpText := `
Usage: serf force-leave [options] name

  Forces a member of a Serf cluster to enter the "left" state. Note
  that if the member is still actually alive, it will eventually rejoin
  the cluster. This command is most useful for cleaning out "failed" nodes
  that are never coming back. If you do not force leave a failed node,
  Serf will attempt to reconnect to those failed nodes for some period of
  time before eventually reaping them.

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -rpc-auth=""              RPC auth token of the Serf agent.
  -prune                    Remove agent forcibly from list of members
`
	return strings.TrimSpace(helpText)
}
