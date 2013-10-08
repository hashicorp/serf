package cli

// AgentCommand is a Command implementation that runs a Serf agent.
// This command does not return unless a SIGINT is received, which will
// gracefully leave the cluster and stop.
type AgentCommand struct {}

func (c *AgentCommand) Help() string {
	return ""
}

func (c *AgentCommand) Run(_ []string, ui Ui) int {
	return 0
}

func (c *AgentCommand) Synopsis() string {
	return "runs a Serf agent"
}
