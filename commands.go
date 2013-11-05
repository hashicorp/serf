package main

import (
	"github.com/hashicorp/serf/command"
	"github.com/hashicorp/serf/command/agent"
	"github.com/mitchellh/cli"
	"os"
	"os/signal"
)

// Commands is the mapping of all the available Serf commands.
var Commands map[string]cli.CommandFactory

func init() {
	ui := &cli.BasicUi{Writer: os.Stdout}

	Commands = map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			return &agent.Command{
				ShutdownCh: makeShutdownCh(),
				Ui:         ui,
			}, nil
		},

		"event": func() (cli.Command, error) {
			return &command.EventCommand{}, nil
		},

		"force-leave": func() (cli.Command, error) {
			return &command.ForceLeaveCommand{}, nil
		},

		"join": func() (cli.Command, error) {
			return &command.JoinCommand{}, nil
		},

		"keygen": func() (cli.Command, error) {
			return &command.KeygenCommand{}, nil
		},

		"members": func() (cli.Command, error) {
			return &command.MembersCommand{}, nil
		},

		"monitor": func() (cli.Command, error) {
			return &command.MonitorCommand{
				ShutdownCh: makeShutdownCh(),
			}, nil
		},

		"version": func() (cli.Command, error) {
			return &command.VersionCommand{
				Revision:          GitCommit,
				Version:           Version,
				VersionPrerelease: VersionPrerelease,
			}, nil
		},
	}
}

// makeShutdownCh returns a channel that can be used for shutdown
// notifications for commands. This channel will send a message for every
// interrupt received.
func makeShutdownCh() <-chan struct{} {
	resultCh := make(chan struct{})

	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		for {
			<-signalCh
			resultCh <- struct{}{}
		}
	}()

	return resultCh
}
