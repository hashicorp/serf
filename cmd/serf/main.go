package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/mitchellh/cli"
)

func main() {
	log.SetOutput(ioutil.Discard)

	c := cli.NewCLI("serf", "")
	c.Args = os.Args[1:]
	c.Commands = Commands
	c.HelpFunc = cli.BasicHelpFunc("serf")

	exitCode, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}
