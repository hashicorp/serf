package cli

import (
	"sync"
)

// CommandFactory is a type of function that is a factory for commands.
// We need a factory because we may need to setup some state on the
// struct that implements the command itself.
type CommandFactory func(string) (Command, error)

// CLI contains the state necessary to run subcommands and parse the
// command line arguments.
type CLI struct {
	Args     []string
	Commands map[string]CommandFactory
	Ui       Ui

	once   sync.Once
	isHelp bool
}

// IsHelp returns whether or not the help flag is present within the
// arguments.
func (c *CLI) IsHelp() bool {
	c.once.Do(c.processArgs)
	return c.isHelp
}

// Run runs the actual CLI based on the arguments given.
func (c *CLI) Run() (int, error) {
	return 0, nil
}

func (c *CLI) processArgs() {
	for _, arg := range c.Args {
		// If the arg is a help flag, then we saw that, but don't save it.
		if arg == "-h" || arg == "--help" {
			c.isHelp = true
			continue
		}
	}
}
