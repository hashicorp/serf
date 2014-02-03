package agent

import (
	"flag"
	"fmt"
	"github.com/armon/go-metrics"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/cli"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// gracefulTimeout controls how long we wait before forcefully terminating
var gracefulTimeout = 3 * time.Second

// Command is a Command implementation that runs a Serf agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	Ui            cli.Ui
	ShutdownCh    <-chan struct{}
	args          []string
	scriptHandler *ScriptEventHandler
	logFilter     *logutils.LevelFilter
}

// readConfig is responsible for setup of our configuration using
// the command line and any file configs
func (c *Command) readConfig() *Config {
	var cmdConfig Config
	var configFiles []string
	var tags []string
	cmdFlags := flag.NewFlagSet("agent", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.StringVar(&cmdConfig.BindAddr, "bind", "", "address to bind listeners to")
	cmdFlags.StringVar(&cmdConfig.AdvertiseAddr, "advertise", "", "address to advertise to cluster")
	cmdFlags.Var((*AppendSliceValue)(&configFiles), "config-file",
		"json file to read config from")
	cmdFlags.Var((*AppendSliceValue)(&configFiles), "config-dir",
		"directory of json files to read")
	cmdFlags.StringVar(&cmdConfig.EncryptKey, "encrypt", "", "encryption key")
	cmdFlags.Var((*AppendSliceValue)(&cmdConfig.EventHandlers), "event-handler",
		"command to execute when events occur")
	cmdFlags.Var((*AppendSliceValue)(&cmdConfig.StartJoin), "join",
		"address of agent to join on startup")
	cmdFlags.BoolVar(&cmdConfig.ReplayOnJoin, "replay", false,
		"replay events for startup join")
	cmdFlags.StringVar(&cmdConfig.LogLevel, "log-level", "", "log level")
	cmdFlags.StringVar(&cmdConfig.NodeName, "node", "", "node name")
	cmdFlags.IntVar(&cmdConfig.Protocol, "protocol", -1, "protocol version")
	cmdFlags.StringVar(&cmdConfig.Role, "role", "", "role name")
	cmdFlags.StringVar(&cmdConfig.RPCAddr, "rpc-addr", "",
		"address to bind RPC listener to")
	cmdFlags.StringVar(&cmdConfig.Profile, "profile", "", "timing profile to use (lan, wan, local)")
	cmdFlags.StringVar(&cmdConfig.SnapshotPath, "snapshot", "", "path to the snapshot file")
	cmdFlags.Var((*AppendSliceValue)(&tags), "tag",
		"tag pair, specified as key=value")
	cmdFlags.StringVar(&cmdConfig.Discover, "discover", "", "mDNS discovery name")
	if err := cmdFlags.Parse(c.args); err != nil {
		return nil
	}

	// Parse any command line tag values
	cmdConfig.Tags = make(map[string]string)
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if len(parts) != 2 {
			c.Ui.Error(fmt.Sprintf("Invalid tag '%s' provided", tag))
			return nil
		}
		cmdConfig.Tags[parts[0]] = parts[1]
	}

	config := DefaultConfig()
	if len(configFiles) > 0 {
		fileConfig, err := ReadConfigPaths(configFiles)
		if err != nil {
			c.Ui.Error(err.Error())
			return nil
		}

		config = MergeConfig(config, fileConfig)
	}

	config = MergeConfig(config, &cmdConfig)

	if config.NodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error determining hostname: %s", err))
			return nil
		}
		config.NodeName = hostname
	}

	eventScripts := config.EventScripts()
	for _, script := range eventScripts {
		if !script.Valid() {
			c.Ui.Error(fmt.Sprintf("Invalid event script: %s", script.String()))
			return nil
		}
	}

	// Backward compatibility hack for 'Role'
	if config.Role != "" {
		c.Ui.Output("Deprecation warning: 'Role' has been replaced with 'Tags'")
		config.Tags["role"] = config.Role
	}

	return config
}

// setupAgent is used to create the agent we use
func (c *Command) setupAgent(config *Config, logOutput io.Writer) *Agent {
	bindIP, bindPort, err := config.AddrParts(config.BindAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Invalid bind address: %s", err))
		return nil
	}

	var advertiseIP string
	var advertisePort int
	if config.AdvertiseAddr != "" {
		advertiseIP, advertisePort, err = config.AddrParts(config.AdvertiseAddr)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Invalid advertise address: %s", err))
			return nil
		}
	}

	encryptKey, err := config.EncryptBytes()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Invalid encryption key: %s", err))
		return nil
	}

	serfConfig := serf.DefaultConfig()
	switch config.Profile {
	case "lan":
		serfConfig.MemberlistConfig = memberlist.DefaultLANConfig()
	case "wan":
		serfConfig.MemberlistConfig = memberlist.DefaultWANConfig()
	case "local":
		serfConfig.MemberlistConfig = memberlist.DefaultLocalConfig()
	default:
		c.Ui.Error(fmt.Sprintf("Unknown profile: %s", config.Profile))
		return nil
	}

	serfConfig.MemberlistConfig.BindAddr = bindIP
	serfConfig.MemberlistConfig.BindPort = bindPort
	serfConfig.MemberlistConfig.AdvertiseAddr = advertiseIP
	serfConfig.MemberlistConfig.AdvertisePort = advertisePort
	serfConfig.MemberlistConfig.SecretKey = encryptKey
	serfConfig.NodeName = config.NodeName
	serfConfig.Tags = config.Tags
	serfConfig.SnapshotPath = config.SnapshotPath
	serfConfig.ProtocolVersion = uint8(config.Protocol)
	serfConfig.CoalescePeriod = 3 * time.Second
	serfConfig.QuiescentPeriod = time.Second
	serfConfig.UserCoalescePeriod = 3 * time.Second
	serfConfig.UserQuiescentPeriod = time.Second

	// Start Serf
	c.Ui.Output("Starting Serf agent...")
	agent, err := Create(serfConfig, logOutput)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to start the Serf agent: %v", err))
		return nil
	}
	return agent
}

// setupLoggers is used to setup the logGate, logWriter, and our logOutput
func (c *Command) setupLoggers(config *Config) (*GatedWriter, *logWriter, io.Writer) {
	// Setup logging. First create the gated log writer, which will
	// store logs until we're ready to show them. Then create the level
	// filter, filtering logs of the specified level.
	logGate := &GatedWriter{
		Writer: &cli.UiWriter{Ui: c.Ui},
	}

	c.logFilter = LevelFilter()
	c.logFilter.MinLevel = logutils.LogLevel(strings.ToUpper(config.LogLevel))
	c.logFilter.Writer = logGate
	if !ValidateLevelFilter(c.logFilter.MinLevel, c.logFilter) {
		c.Ui.Error(fmt.Sprintf(
			"Invalid log level: %s. Valid log levels are: %v",
			c.logFilter.MinLevel, c.logFilter.Levels))
		return nil, nil, nil
	}

	// Create a log writer, and wrap a logOutput around it
	logWriter := NewLogWriter(512)
	logOutput := io.MultiWriter(c.logFilter, logWriter)
	return logGate, logWriter, logOutput
}

// startAgent is used to start the agent and IPC
func (c *Command) startAgent(config *Config, agent *Agent,
	logWriter *logWriter, logOutput io.Writer) *AgentIPC {
	// Add the script event handlers
	c.scriptHandler = &ScriptEventHandler{
		Self: serf.Member{
			Name: config.NodeName,
			Tags: config.Tags,
		},
		Scripts: config.EventScripts(),
		Logger:  log.New(logOutput, "", log.LstdFlags),
	}
	agent.RegisterEventHandler(c.scriptHandler)

	// Start the agent after the handler is registered
	if err := agent.Start(); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to start the Serf agent: %v", err))
		return nil
	}

	// Parse the bind address information
	bindIP, bindPort, err := config.AddrParts(config.BindAddr)
	bindAddr := &net.TCPAddr{IP: net.ParseIP(bindIP), Port: bindPort}

	// Start the discovery layer
	if config.Discover != "" {
		// Use the advertise addr and port
		local := agent.Serf().Memberlist().LocalNode()

		_, err := NewAgentMDNS(agent, logOutput, config.ReplayOnJoin,
			config.NodeName, config.Discover, local.Addr, int(local.Port))
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error starting mDNS listener: %s", err))
			return nil

		}
	}

	// Setup the RPC listener
	rpcListener, err := net.Listen("tcp", config.RPCAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error starting RPC listener: %s", err))
		return nil
	}

	// Start the IPC layer
	c.Ui.Output("Starting Serf agent RPC...")
	ipc := NewAgentIPC(agent, rpcListener, logOutput, logWriter)

	c.Ui.Output("Serf agent running!")
	c.Ui.Info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	c.Ui.Info(fmt.Sprintf("     Bind addr: '%s'", bindAddr.String()))

	if config.AdvertiseAddr != "" {
		advertiseIP, advertisePort, _ := config.AddrParts(config.AdvertiseAddr)
		advertiseAddr := (&net.TCPAddr{IP: net.ParseIP(advertiseIP), Port: advertisePort}).String()
		c.Ui.Info(fmt.Sprintf("Advertise addr: '%s'", advertiseAddr))
	}

	c.Ui.Info(fmt.Sprintf("      RPC addr: '%s'", config.RPCAddr))
	c.Ui.Info(fmt.Sprintf("     Encrypted: %#v", config.EncryptKey != ""))
	c.Ui.Info(fmt.Sprintf("      Snapshot: %v", config.SnapshotPath != ""))
	c.Ui.Info(fmt.Sprintf("       Profile: %s", config.Profile))

	if config.Discover != "" {
		c.Ui.Info(fmt.Sprintf("  mDNS cluster: %s", config.Discover))
	}
	return ipc
}

// startupJoin is invoked to handle any joins specified to take place at start time
func (c *Command) startupJoin(config *Config, agent *Agent) error {
	if len(config.StartJoin) == 0 {
		return nil
	}

	c.Ui.Output(fmt.Sprintf("Joining cluster...(replay: %v)", config.ReplayOnJoin))
	n, err := agent.Join(config.StartJoin, config.ReplayOnJoin)
	if err != nil {
		return err
	}

	c.Ui.Info(fmt.Sprintf("Join completed. Synced with %d initial agents", n))
	return nil
}

func (c *Command) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}

	// Parse our configs
	c.args = args
	config := c.readConfig()
	if config == nil {
		return 1
	}

	// Setup the log outputs
	logGate, logWriter, logOutput := c.setupLoggers(config)
	if logWriter == nil {
		return 1
	}

	/* Setup telemetry
	Aggregate on 10 second intervals for 1 minute. Expose the
	metrics over stderr when there is a SIGUSR1 received.
	*/
	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(inm)
	metricsConf := metrics.DefaultConfig("serf-agent")
	metricsConf.EnableHostname = false
	metrics.NewGlobal(metricsConf, inm)

	// Setup serf
	agent := c.setupAgent(config, logOutput)
	if agent == nil {
		return 1
	}
	defer agent.Shutdown()

	// Start the agent
	ipc := c.startAgent(config, agent, logWriter, logOutput)
	if ipc == nil {
		return 1
	}
	defer ipc.Shutdown()

	// Join startup nodes if specified
	if err := c.startupJoin(config, agent); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Enable log streaming
	c.Ui.Info("")
	c.Ui.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// Wait for exit
	return c.handleSignals(config, agent)
}

// handleSignals blocks until we get an exit-causing signal
func (c *Command) handleSignals(config *Config, agent *Agent) int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Wait for a signal
WAIT:
	var sig os.Signal
	select {
	case s := <-signalCh:
		sig = s
	case <-c.ShutdownCh:
		sig = os.Interrupt
	case <-agent.ShutdownCh():
		// Agent is already shutdown!
		return 0
	}
	c.Ui.Output(fmt.Sprintf("Caught signal: %v", sig))

	// Check if this is a SIGHUP
	if sig == syscall.SIGHUP {
		config = c.handleReload(config, agent)
		goto WAIT
	}

	// Check if we should do a graceful leave
	graceful := false
	if sig == os.Interrupt && !config.SkipLeaveOnInt {
		graceful = true
	} else if sig == syscall.SIGTERM && config.LeaveOnTerm {
		graceful = true
	}

	// Bail fast if not doing a graceful leave
	if !graceful {
		return 1
	}

	// Attempt a graceful leave
	gracefulCh := make(chan struct{})
	c.Ui.Output("Gracefully shutting down agent...")
	go func() {
		if err := agent.Leave(); err != nil {
			c.Ui.Error(fmt.Sprintf("Error: %s", err))
			return
		}
		close(gracefulCh)
	}()

	// Wait for leave or another signal
	select {
	case <-signalCh:
		return 1
	case <-time.After(gracefulTimeout):
		return 1
	case <-gracefulCh:
		return 0
	}
}

// handleReload is invoked when we should reload our configs, e.g. SIGHUP
func (c *Command) handleReload(config *Config, agent *Agent) *Config {
	c.Ui.Output("Reloading configuration...")
	newConf := c.readConfig()
	if newConf == nil {
		c.Ui.Error(fmt.Sprintf("Failed to reload configs"))
		return config
	}

	// Change the log level
	minLevel := logutils.LogLevel(strings.ToUpper(newConf.LogLevel))
	if ValidateLevelFilter(minLevel, c.logFilter) {
		c.logFilter.SetMinLevel(minLevel)
	} else {
		c.Ui.Error(fmt.Sprintf(
			"Invalid log level: %s. Valid log levels are: %v",
			minLevel, c.logFilter.Levels))

		// Keep the current log level
		newConf.LogLevel = config.LogLevel
	}

	// Change the event handlers
	c.scriptHandler.UpdateScripts(config.EventScripts())

	// Update the tags in serf
	serf := agent.Serf()
	if err := serf.SetTags(newConf.Tags); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to update tags: %v", err))
		return newConf
	}

	// Change the tags for the event handlers
	c.scriptHandler.Self.Tags = newConf.Tags

	return newConf
}

func (c *Command) Synopsis() string {
	return "Runs a Serf agent"
}

func (c *Command) Help() string {
	helpText := `
Usage: serf agent [options]

  Starts the Serf agent and runs until an interrupt is received. The
  agent represents a single node in a cluster.

Options:

  -bind=0.0.0.0            Address to bind network listeners to
  -advertise=0.0.0.0       Address to advertise to the other cluster members
  -config-file=foo         Path to a JSON file to read configuration from.
                           This can be specified multiple times.
  -config-dir=foo          Path to a directory to read configuration files
                           from. This will read every file ending in ".json"
                           as configuration in this directory in alphabetical
                           order.
  -discover=cluster        Discover is set to enable mDNS discovery of peer. On
                           networks that support multicast, this can be used to have
                           peers join each other without an explicit join.
  -encrypt=foo             Key for encrypting network traffic within Serf.
                           Must be a base64-encoded 16-byte key.
  -event-handler=foo       Script to execute when events occur. This can
                           be specified multiple times. See the event scripts
                           section below for more info.
  -join=addr               An initial agent to join with. This flag can be
                           specified multiple times.
  -log-level=info          Log level of the agent.
  -node=hostname           Name of this node. Must be unique in the cluster
  -profile=[lan|wan|local] Profile is used to control the timing profiles used in Serf.
						   The default if not provided is lan.
  -protocol=n              Serf protocol version to use. This defaults to
                           the latest version, but can be set back for upgrades.
  -role=foo                The role of this node, if any. This can be used
                           by event scripts to differentiate different types
                           of nodes that may be part of the same cluster.
                           '-role' is deprecated in favor of '-tag role=foo'.
  -rpc-addr=127.0.0.1:7373 Address to bind the RPC listener.
  -snapshot=path/to/file   The snapshot file is used to store alive nodes and
                           event information so that Serf can rejoin a cluster
						   and avoid event replay on restart.
  -tag key=value           Tag can be specified multiple times to attach multiple
                           key/value tag pairs to the given node.

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
