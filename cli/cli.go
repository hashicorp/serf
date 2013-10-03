package cli

// CommandFactory is a type of function that is a factory for commands.
// We need a factory because we may need to setup some state on the
// struct that implements the command itself.
type CommandFactory func(string) (Command, error)

// CLI contains the state necessary to run subcommands and parse the
// command line arguments.
type CLI struct {
	Args     []string
	Commands map[string]CommandFactory
}

// Run runs the actual CLI based on the arguments given.
func (c *CLI) Run() (int, error) {
	return 0, nil
}
