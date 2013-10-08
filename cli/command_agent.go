package cli

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
)

// AgentCommand is a Command implementation that runs a Serf agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type AgentCommand struct {
	ShutdownCh <-chan struct{}
}

func (c *AgentCommand) Help() string {
	return ""
}

func (c *AgentCommand) Run(_ []string, ui Ui) int {
	config := &serf.Config{}
	serf, err := serf.Create(config)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to initialize Serf: %s", err))
		return 1
	}

	graceful, forceful := c.startShutdownWatcher(serf, ui)
	select {
	case <-graceful:
	case <-forceful:
		// Forcefully shut down, return a bad exit status.
		return 1
	}

	return 0
}

func (c *AgentCommand) Synopsis() string {
	return "runs a Serf agent"
}

func (c *AgentCommand) forceShutdown(serf *serf.Serf, ui Ui) {
	ui.Output("Forcefully shutting down agent...")
	if err := serf.Shutdown(); err != nil {
		ui.Error(fmt.Sprintf("Error: %s", err))
	}
}

func (c *AgentCommand) gracefulShutdown(serf *serf.Serf, ui Ui, done chan<- struct{}) {
	ui.Output("Gracefully shutting down agent. " +
		"Interrupt again to forcefully shut down.")
	if err := serf.Leave(); err != nil {
		ui.Error(fmt.Sprintf("Error: %s", err))
		return
	}
	close(done)
}

func (c *AgentCommand) startShutdownWatcher(serf *serf.Serf, ui Ui) (graceful <-chan struct{}, forceful <-chan struct{}) {
	g := make(chan struct{})
	f := make(chan struct{})
	graceful = g
	forceful = f

	go func() {
		<-c.ShutdownCh
		go c.gracefulShutdown(serf, ui, g)

		select {
		case <-g:
			// Gracefully shut down properly
		case <-c.ShutdownCh:
			c.forceShutdown(serf, ui)
			close(f)
		}
	}()

	return
}
