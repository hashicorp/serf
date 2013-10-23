package agent

import (
	"flag"
	"fmt"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/cli"
	"github.com/hashicorp/serf/serf"
	"os"
	"strings"
	"sync"
)

// Command is a Command implementation that runs a Serf agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	ShutdownCh <-chan struct{}

	lock         sync.Mutex
	shuttingDown bool
}

func (c *Command) Help() string {
	helpText := `
Usage: serf agent [options]

  Starts the Serf agent and runs until an interrupt is received. The
  agent represents a single node in a cluster.

Options:

  -bind=0.0.0.0            Address to bind network listeners to
  -event-handler=foo       Script to execute when events occur. This can
                           be specified multiple times. See the event scripts
                           section below for more info.
  -log-level=info          Log level of the agent.
  -node=hostname           Name of this node. Must be unique in the cluster
  -role=foo                The role of this node, if any. This can be used
                           by event scripts to differentiate different types
                           of nodes that may be part of the same cluster.
  -rpc-addr=127.0.0.1:7373 Address to bind the RPC listener.

Event handlers:

  For more information on what event handlers are, please read the
  Serf documentation. This section will document how to configure them
  on the command-line. There are three methods of specifying an event
  handler:

  - The value can be a plain script, such as "event.sh". In this case,
    Serf will send all events to this script, and you'll be responsible
    for differentiating between them based on the SERF_EVENT.

  - The value can be in the format of "TYPE=SCRIPT", such as
    "member-join=join.sh". With this format, Serf will only send events
    of that type to that script.

  - The value can be in the format of "user:EVENT=SCRIPT", such as
    "user:deploy=deploy.sh". This means that Serf will only invoke this
    script in the case of user events named "deploy".
`
	return strings.TrimSpace(helpText)
}

func (c *Command) Run(args []string, rawUi cli.Ui) int {
	ui := &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           rawUi,
	}

	var bindAddr string
	var logLevel string
	var eventHandlers []string
	var nodeName string
	var nodeRole string
	var rpcAddr string

	cmdFlags := flag.NewFlagSet("agent", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.StringVar(&bindAddr, "bind", "0.0.0.0", "address to bind listeners to")
	cmdFlags.Var((*AppendSliceValue)(&eventHandlers), "event-handler",
		"command to execute when events occur")
	cmdFlags.StringVar(&logLevel, "log-level", "INFO", "log level")
	cmdFlags.StringVar(&nodeName, "node", "", "node name")
	cmdFlags.StringVar(&nodeRole, "role", "", "role name")
	cmdFlags.StringVar(&rpcAddr, "rpc-addr", "127.0.0.1:7373",
		"address to bind RPC listener to")
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	if nodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			rawUi.Error(fmt.Sprintf("Error determining hostname: %s", err))
			return 1
		}

		nodeName = hostname
	}

	config := Config{
		NodeName:      nodeName,
		Role:          nodeRole,
		BindAddr:      bindAddr,
		RPCAddr:       rpcAddr,
		EventHandlers: eventHandlers,
	}

	eventScripts, err := config.EventScripts()
	if err != nil {
		rawUi.Error(err.Error())
		return 1
	}

	for _, script := range eventScripts {
		if !script.Valid() {
			rawUi.Error(fmt.Sprintf("Invalid event script: %s", script.String()))
			return 1
		}
	}

	bindIP, bindPort, err := config.BindAddrParts()
	if err != nil {
		rawUi.Error(fmt.Sprintf("Invalid bind address: %s", err))
		return 1
	}

	// Setup logging. First create the gated log writer, which will
	// store logs until we're ready to show them. Then create the level
	// filter, filtering logs of the specified level.
	logGate := &GatedWriter{
		Writer: &cli.UiWriter{Ui: rawUi},
	}

	logLevelFilter := LevelFilter()
	logLevelFilter.MinLevel = logutils.LogLevel(strings.ToUpper(logLevel))
	logLevelFilter.Writer = logGate
	if !ValidateLevelFilter(logLevelFilter) {
		ui.Error(fmt.Sprintf(
			"Invalid log level: %s. Valid log levels are: %v",
			logLevelFilter.MinLevel, logLevelFilter.Levels))
		return 1
	}

	serfConfig := serf.DefaultConfig()
	serfConfig.MemberlistConfig.BindAddr = bindIP
	serfConfig.MemberlistConfig.TCPPort = bindPort
	serfConfig.MemberlistConfig.UDPPort = bindPort
	serfConfig.NodeName = nodeName
	serfConfig.Role = nodeRole

	agent := &Agent{
		EventHandler: &ScriptEventHandler{
			Self: serf.Member{
				Name: serfConfig.NodeName,
				Role: serfConfig.Role,
			},
			Scripts: eventScripts,
		},
		LogOutput:  logLevelFilter,
		RPCAddr:    rpcAddr,
		SerfConfig: serfConfig,
	}

	ui.Output("Starting Serf agent...")
	if err := agent.Start(); err != nil {
		ui.Error(err.Error())
		return 1
	}

	ui.Output("Serf agent running!")
	ui.Info(fmt.Sprintf("Node name: '%s'", config.NodeName))
	ui.Info(fmt.Sprintf("Bind addr: '%s:%d'", bindIP, bindPort))
	ui.Info(fmt.Sprintf(" RPC addr: '%s'", rpcAddr))
	ui.Info("")
	ui.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	graceful, forceful := c.startShutdownWatcher(agent, ui)
	select {
	case <-graceful:
	case <-forceful:
		// Forcefully shut down, return a bad exit status.
		return 1
	}

	return 0
}

func (c *Command) Synopsis() string {
	return "Runs a Serf agent"
}

func (c *Command) startShutdownWatcher(agent *Agent, ui cli.Ui) (graceful <-chan struct{}, forceful <-chan struct{}) {
	g := make(chan struct{})
	f := make(chan struct{})
	graceful = g
	forceful = f

	go func() {
		<-c.ShutdownCh

		c.lock.Lock()
		c.shuttingDown = true
		c.lock.Unlock()

		ui.Output("Gracefully shutting down agent...")
		go func() {
			if err := agent.Shutdown(); err != nil {
				ui.Error(fmt.Sprintf("Error: %s", err))
				return
			}
			close(g)
		}()

		select {
		case <-g:
			// Gracefully shut down properly
		case <-c.ShutdownCh:
			close(f)
		}
	}()

	return
}
