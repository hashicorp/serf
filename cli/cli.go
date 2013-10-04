package cli

import (
	"sync"
)

// CLI contains the state necessary to run subcommands and parse the
// command line arguments.
type CLI struct {
	Args     []string
	Commands map[string]CommandFactory
	Ui       Ui

	once           sync.Once
	isHelp         bool
	subcommand     string
	subcommandArgs []string
}

// IsHelp returns whether or not the help flag is present within the
// arguments.
func (c *CLI) IsHelp() bool {
	c.once.Do(c.processArgs)
	return c.isHelp
}

// Run runs the actual CLI based on the arguments given.
func (c *CLI) Run() (int, error) {
	// If we've been instructed to just print the help, then print it
	if c.IsHelp() {
		c.printHelp()
		return 1, nil
	}

	// Attempt to get the factory function for creating the command
	// implementation. If the command is invalid or blank, it is an error.
	commandFunc, ok := c.Commands[c.Subcommand()]
	if !ok || c.Subcommand() == "" {
		c.printHelp()
		return 1, nil
	}

	command, err := commandFunc()
	if err != nil {
		return 0, err
	}

	return command.Run(c.SubcommandArgs(), c.Ui), nil
}

// Subcommand returns the subcommand that the CLI would execute. For
// example, a CLI from "--version version --help" would return a Subcommand
// of "version"
func (c *CLI) Subcommand() string {
	c.once.Do(c.processArgs)
	return c.subcommand
}

// SubcommandArgs returns the arguments that will be passed to the
// subcommand.
func (c *CLI) SubcommandArgs() []string {
	c.once.Do(c.processArgs)
	return c.subcommandArgs
}

func (c *CLI) printHelp() {
	c.Ui.Error("usage: serf [--version] [--help] <command> [<args>]\n")
	c.Ui.Error("Available commands are:")

	// TODO(mitchellh)
}

func (c *CLI) processArgs() {
	for i, arg := range c.Args {
		// If the arg is a help flag, then we saw that, but don't save it.
		if arg == "-h" || arg == "--help" {
			c.isHelp = true
			continue
		}

		// If we didn't find a subcommand yet and this is the first non-flag
		// argument, then this is our subcommand. j
		if c.subcommand == "" && arg[0] != '-' {
			c.subcommand = arg

			// The remaining args the subcommand arguments
			c.subcommandArgs = c.Args[i+1:]
		}
	}
}
