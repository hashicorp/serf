package serf

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/testutil"
	"strings"
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

func testKeyringSerf() (*Serf, error) {
	config := testConfig()

	keyring, err := testKeyring()
	if err != nil {
		return nil, err
	}
	config.MemberlistConfig.Keyring = keyring

	s, err := Create(config)
	if err != nil {
		return nil, err
	}

	return s, nil
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
	s1, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s1.Shutdown()

	s2, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s2.Shutdown()

	primaryKey := s1.config.MemberlistConfig.Keyring.GetPrimaryKey()

	// Join s1 and s2
	_, err = s1.Join([]string{s2.config.MemberlistConfig.BindAddr}, false)
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

func TestSerf_UseKey(t *testing.T) {
	s1, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s1.Shutdown()

	s2, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s2.Shutdown()

	primaryKey := s1.config.MemberlistConfig.Keyring.GetPrimaryKey()

	// Join s1 and s2
	_, err = s1.Join([]string{s2.config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Begin tests
	useKey := "csT9mxI7aTf9ap3HLBbdmA=="
	useKeyBytes, err := base64.StdEncoding.DecodeString(useKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// First make sure that the primary key is what we expect it to be
	if !bytes.Equal(primaryKey, s1.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key on s1")
	}

	if !bytes.Equal(primaryKey, s2.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key on s2")
	}

	// Change the primary encryption key
	resp := s1.UseKey(useKey)
	if resp.Err != nil {
		t.Fatalf("err: %s", resp.Err)
	}

	// First make sure that the primary key is what we expect it to be
	if !bytes.Equal(useKeyBytes, s1.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key on s1")
	}

	if !bytes.Equal(useKeyBytes, s2.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key on s2")
	}

	// Make sure an error is thrown if the key doesn't exist
	resp = s1.UseKey("aE6AfGEvay+UJbkfxBk4SQ==")
	if resp.Err == nil {
		t.Fatalf("Expected error changing to non-existent primary key")
	}
}

func TestSerf_RemoveKey(t *testing.T) {
	s1, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s1.Shutdown()

	s2, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s2.Shutdown()

	// Join s1 and s2
	_, err = s1.Join([]string{s2.config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Begin tests
	removeKey := "noha2tVc0OyD/2LtCBoAOQ=="
	removeKeyBytes, err := base64.StdEncoding.DecodeString(removeKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Change the primary encryption key
	resp := s1.RemoveKey(removeKey)
	if resp.Err != nil {
		t.Fatalf("err: %s", resp.Err)
	}

	// Key was successfully removed from all members
	if keyExistsInRing(s1.config.MemberlistConfig.Keyring, removeKeyBytes) {
		t.Fatal("Key not removed from keyring on s1")
	}

	if keyExistsInRing(s2.config.MemberlistConfig.Keyring, removeKeyBytes) {
		t.Fatal("Key not removed from keyring on s2")
	}
}
