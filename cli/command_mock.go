package cli

// MockCommand is an implementation of Command that can be used for tests.
// It is publicly exported from this package in case you want to use it
// externally.
type MockCommand struct {
	// Settable
	RunResult int

	// Set by the command
	RunCalled bool
	RunArgs   []string
	RunUi     Ui
}

func (c *MockCommand) Help() string {
	return ""
}

func (c *MockCommand) Run(args []string, ui Ui) int {
	c.RunCalled = true
	c.RunArgs = args
	c.RunUi = ui

	return c.RunResult
}

func (c *MockCommand) Synopsis() string {
	return ""
}
