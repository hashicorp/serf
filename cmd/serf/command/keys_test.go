package command

import (
	"encoding/base64"
	"net"
	"strings"
	"testing"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"github.com/mitchellh/cli"
)

func testKeysCommandAgent(t *testing.T, ip net.IP) *agent.Agent {
	key1, err := base64.StdEncoding.DecodeString("ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	key2, err := base64.StdEncoding.DecodeString("WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	keyring, err := memberlist.NewKeyring([][]byte{key1, key2}, key1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	agentConf := agent.DefaultConfig()
	serfConf := serf.DefaultConfig()
	serfConf.MemberlistConfig.Keyring = keyring

	a1 := testAgentWithConfig(t, ip, agentConf, serfConf)
	return a1
}

func TestKeysCommandRun_InstallKey(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testKeysCommandAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	rpcClient, err := client.NewRPCClient(rpcAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	keys, _, _, err := rpcClient.ListKeys()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := keys["HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="]; ok {
		t.Fatalf("have test key")
	}

	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-install", "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8=",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "Successfully installed key") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	keys, _, _, err = rpcClient.ListKeys()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := keys["HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="]; !ok {
		t.Fatalf("new key not found")
	}
}

func TestKeysCommandRun_InstallKeyFailure(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	// Trying to install with encryption disabled returns 1
	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-install", "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8=",
	}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Node errors appear in stderr
	if !strings.Contains(ui.ErrorWriter.String(), "not enabled") {
		t.Fatalf("expected empty keyring error")
	}
}

func TestKeysCommandRun_UseKey(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testKeysCommandAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	// Trying to use a non-existent key returns 1
	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-use", "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s=",
	}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Using an existing key returns 0
	args = []string{
		"-rpc-addr=" + rpcAddr,
		"-use", "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=",
	}

	code = c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}

func TestKeysCommandRun_UseKeyFailure(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testKeysCommandAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	// Trying to use a key that doesn't exist returns 1
	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-use", "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8=",
	}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Node errors appear in stderr
	if !strings.Contains(ui.ErrorWriter.String(), "not in the keyring") {
		t.Fatalf("expected absent key error")
	}
}

func TestKeysCommandRun_RemoveKey(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testKeysCommandAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	rpcClient, err := client.NewRPCClient(rpcAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	keys, _, _, err := rpcClient.ListKeys()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys: %v", keys)
	}

	// Removing non-existing key still returns 0 (noop)
	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-remove", "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s=",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Number of keys unchanged after noop command
	keys, _, _, err = rpcClient.ListKeys()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys: %v", keys)
	}

	// Removing a primary key returns 1
	args = []string{
		"-rpc-addr=" + rpcAddr,
		"-remove", "ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=",
	}

	ui.ErrorWriter.Reset()
	code = c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.ErrorWriter.String(), "Error removing key") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	// Removing a non-primary, existing key returns 0
	args = []string{
		"-rpc-addr=" + rpcAddr,
		"-remove", "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=",
	}

	code = c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Key removed after successful -remove command
	keys, _, _, err = rpcClient.ListKeys()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 2 keys: %v", keys)
	}
}

func TestKeysCommandRun_RemoveKeyFailure(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testKeysCommandAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	// Trying to remove the primary key returns 1
	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-remove", "ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=",
	}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Node errors appear in stderr
	if !strings.Contains(ui.ErrorWriter.String(), "not allowed") {
		t.Fatalf("expected primary key removal error")
	}
}

func TestKeysCommandRun_ListKeys(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testKeysCommandAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-list",
	}

	code := c.Run(args)
	if code == 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=") {
		t.Fatalf("missing expected key")
	}

	if !strings.Contains(ui.OutputWriter.String(), "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=") {
		t.Fatalf("missing expected key")
	}
}

func TestKeysCommandRun_ListKeysFailure(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	// Trying to list keys with encryption disabled returns 1
	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-list",
	}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.ErrorWriter.String(), "not enabled") {
		t.Fatalf("expected empty keyring error")
	}
}

func TestKeysCommandRun_BadOptions(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	a1 := testAgent(t, ip1)
	defer a1.Shutdown()

	rpcAddr, ipc := testIPC(t, ip2, a1)
	defer ipc.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}

	args := []string{
		"-rpc-addr=" + rpcAddr,
		"-install", "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=",
		"-use", "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=",
	}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	args = []string{
		"-rpc-addr=" + rpcAddr,
		"-list",
		"-remove", "ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=",
	}

	code = c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
}
