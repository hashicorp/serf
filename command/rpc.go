package command

import (
	"flag"
	"github.com/hashicorp/serf/client"
)

// RPCAddrFlag returns a pointer to a string that will be populated
// when the given flagset is parsed with the RPC address of the Serf.
func RPCAddrFlag(f *flag.FlagSet) *string {
	return f.String("rpc-addr", "127.0.0.1:7373",
		"RPC address of the Serf agent")
}

// RPCAuthFlag returns a pointer to a string that will be populated
// when the given flagset is parsed with the RPC auth token of the Serf.
func RPCAuthFlag(f *flag.FlagSet) *string {
	return f.String("rpc-auth", "",
		"RPC auth token of the Serf agent")
}

// RPCClient returns a new Serf RPC client with the given address.
func RPCClient(addr, auth string) (*client.RPCClient, error) {
	config := client.Config{Addr: addr, AuthKey: auth}
	return client.ClientFromConfig(&config)
}
