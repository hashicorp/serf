package agent

// Config is the configuration that can be set for an Agent. Some of these
// configurations are exposed as command-line flags to `serf agent`, whereas
// many of the more advanced configurations can only be set by creating
// a configuration file.
type Config struct {
	// All the configurations in this section are identical to their
	// Serf counterparts. See the documentation for Serf.Config for
	// more info.
	NodeName string `mapstructure:"node_name"`
	Role     string `mapstructure:"role"`

	// BindAddr is the address that the Serf agent's communication ports
	// will bind to. Serf may use multiple ports (see documentation), so
	// this is only the address to bind to.
	BindAddr string `mapstructure:"bind_addr"`

	// RPCAddr is the address and port to listen on for the agent's RPC
	// interface.
	RPCAddr string `mapstructure:"rpc_addr"`

	// EventHandlers is a list of event handlers that will be invoked.
	EventHandlers []string `mapstructure:"event_handlers"`
}

// EventScripts returns the list of EventScripts associated with this
// configuration and specified by the "event_handlers" configuration.
func (c *Config) EventScripts() ([]EventScript, error) {
	result := make([]EventScript, 0, len(c.EventHandlers))
	for _, v := range c.EventHandlers {
		part, err := ParseEventScript(v)
		if err != nil {
			return nil, err
		}

		result = append(result, part...)
	}

	return result, nil
}
