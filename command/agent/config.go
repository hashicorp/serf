package agent

import (
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// This is the default port that we use for Serf communication
const DefaultBindPort int = 7946

// DefaultConfig contains the defaults for configurations.
var DefaultConfig = &Config{
	BindAddr: "0.0.0.0",
	LogLevel: "INFO",
	RPCAddr:  "127.0.0.1:7373",
}

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
	// will bind to. Serf will use this address to bind to for both TCP
	// and UDP connections. If no port is present in the address, the default
	// port will be used.
	BindAddr string `mapstructure:"bind_addr"`

	// LogLevel is the level of the logs to output.
	LogLevel string `mapstructure:"log_level"`

	// RPCAddr is the address and port to listen on for the agent's RPC
	// interface.
	RPCAddr string `mapstructure:"rpc_addr"`

	// EventHandlers is a list of event handlers that will be invoked.
	EventHandlers []string `mapstructure:"event_handlers"`
}

// BindAddrParts returns the parts of the BindAddr that should be
// used to configure Serf.
func (c *Config) BindAddrParts() (string, int, error) {
	checkAddr := c.BindAddr
	if !strings.Contains(checkAddr, ":") {
		checkAddr += fmt.Sprintf(":%d", DefaultBindPort)
	}

	addr, err := net.ResolveTCPAddr("tcp", checkAddr)
	if err != nil {
		return "", 0, err
	}

	return addr.IP.String(), addr.Port, nil
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

// DecodeConfig reads the configuration from the given reader in JSON
// format and decodes it into a proper Config structure.
func DecodeConfig(r io.Reader) (*Config, error) {
	var raw interface{}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}

	var result Config
	if err := mapstructure.Decode(raw, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// MergeConfig merges two configurations together to make a single new
// configuration.
func MergeConfig(a, b *Config) *Config {
	var result Config = *a

	// Copy the strings if they're set
	if b.NodeName != "" {
		result.NodeName = b.NodeName
	}
	if b.Role != "" {
		result.Role = b.Role
	}
	if b.BindAddr != "" {
		result.BindAddr = b.BindAddr
	}
	if b.LogLevel != "" {
		result.LogLevel = b.LogLevel
	}
	if b.RPCAddr != "" {
		result.RPCAddr = b.RPCAddr
	}

	// Copy the event handlers
	result.EventHandlers = make([]string, 0, len(a.EventHandlers)+len(b.EventHandlers))
	result.EventHandlers = append(result.EventHandlers, a.EventHandlers...)
	result.EventHandlers = append(result.EventHandlers, b.EventHandlers...)

	return &result
}

// ReadConfigPaths reads the paths in the given order to load configurations.
// The paths can be to files or directories. If the path is a directory,
// we read one directory deep and read any files ending in ".json" as
// configuration files.
func ReadConfigPaths(paths []string) (*Config, error) {
	result := new(Config)
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		if !fi.IsDir() {
			config, err := DecodeConfig(f)
			f.Close()

			if err != nil {
				return nil, fmt.Errorf("Error decoding '%s': %s", path, err)
			}

			result = MergeConfig(result, config)
			continue
		}

		contents, err := f.Readdir(-1)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		for _, fi := range contents {
			// Don't recursively read contents
			if fi.IsDir() {
				continue
			}

			// If it isn't a JSON file, ignore it
			if !strings.HasSuffix(fi.Name(), ".json") {
				continue
			}

			subpath := filepath.Join(path, fi.Name())
			f, err := os.Open(subpath)
			if err != nil {
				return nil, fmt.Errorf("Error reading '%s': %s", subpath, err)
			}

			config, err := DecodeConfig(f)
			f.Close()

			if err != nil {
				return nil, fmt.Errorf("Error decoding '%s': %s", subpath, err)
			}

			result = MergeConfig(result, config)
		}
	}

	return result, nil
}
