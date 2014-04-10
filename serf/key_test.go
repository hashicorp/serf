package serf

import (
	"bytes"
	"encoding/base64"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/testutil"
	"testing"
)

func testKeyring() (*memberlist.Keyring, error) {
	keys := []string{
		"enjTwAFRe4IE71bOFhirzQ==",
		"csT9mxI7aTf9ap3HLBbdmA==",
		"noha2tVc0OyD/2LtCBoAOQ==",
	}

	keysDecoded := make([][]byte, len(keys))
	for i, key := range keys {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return nil, err
		}
		keysDecoded[i] = decoded
	}

	return memberlist.NewKeyring(keysDecoded, keysDecoded[0])
}

func keyExistsInRing(kr *memberlist.Keyring, key []byte) bool {
	for _, installedKey := range kr.GetKeys() {
		if bytes.Equal(key, installedKey) {
			return true
		}
	}
	return false
}

func TestSerf_InstallKey(t *testing.T) {
	// Create s1
	s1Config := testConfig()

	keyring, err := testKeyring()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	s1Config.MemberlistConfig.Keyring = keyring
	primaryKey := keyring.GetPrimaryKey()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	// Create s2
	s2Config := testConfig()

	keyring, err = testKeyring()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	s2Config.MemberlistConfig.Keyring = keyring

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	// Join s1 and s2
	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Begin tests
	newKey := "l4ZkaypGLT8AsB0LBldthw=="
	newKeyBytes, err := base64.StdEncoding.DecodeString(newKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Install a new key onto the existing ring. This is a blocking call, so no
	// need for a yield.
	resp := s1.InstallKey(newKey)
	if resp.Err != nil {
		t.Fatalf("err: %s", resp.Err)
	}

	// Key installation did not affect the current primary key
	if !bytes.Equal(primaryKey, s1.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key change on s1")
	}

	if !bytes.Equal(primaryKey, s2.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key change on s2")
	}

	// New key was successfully broadcasted and installed on all members
	if !keyExistsInRing(s1.config.MemberlistConfig.Keyring, newKeyBytes) {
		t.Fatal("Newly-installed key not found in keyring on s1")
	}

	if !keyExistsInRing(s2.config.MemberlistConfig.Keyring, newKeyBytes) {
		t.Fatal("Newly-installed key not found in keyring on s2")
	}
}
