package main

import (
	"github.com/hashicorp/serf/cli"
	"io/ioutil"
	"log"
	"os"
)

// The git commit that was compiled. This will be filled in by the compiler.
var GitCommit string

// The main version number that is being run at the moment.
const Version = "0.1.0"

// A pre-release marker for the version. If this is "" (empty string)
// then it means that it is a final release. Otherwise, this is a pre-release
// such as "dev" (in development), "beta", "rc1", etc.
const VersionPrerelease = "dev"

func main() {
	os.Exit(realMain())
}

func realMain() int {
	log.SetOutput(ioutil.Discard)
	if os.Getenv("SERF_LOG") != "" {
		log.SetOutput(os.Stderr)
	}

	cli := &cli.CLI{
		Args:     os.Args[1:],
		Commands: make(map[string]cli.CommandFactory),
		Ui:       &cli.BasicUi{Writer: os.Stdout},
	}

	exitCode, err := cli.Run()
	if err != nil {
		// TODO(mitchellh): too harsh
		panic(err)
	}

	return exitCode
}
