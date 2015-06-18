package coordinate

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClient_NewClient(t *testing.T) {
	config := DefaultConfig()

	config.Dimensionality = 0
	client, err := NewClient(config)
	if err == nil || !strings.Contains(err.Error(), "dimensionality") {
		t.Fatal(err)
	}

	config.Dimensionality = 7
	client, err = NewClient(config)
	if err != nil {
		t.Fatal(err)
	}

	origin := NewCoordinate(config)
	if !reflect.DeepEqual(client.GetCoordinate(), origin) {
		t.Fatalf("fresh client should be located at the origin")
	}
}

func TestClient_Update(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3

	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the Euclidian part of our coordinate is what we expect.
	c := client.GetCoordinate()
	verifyEqualVectors(t, c.Vec, []float64{0.0, 0.0, 0.0})

	// Place a node right above the client and observe an RTT longer than the
	// client expects, given its distance.
	other := NewCoordinate(config)
	other.Vec[2] = 0.001
	rtt := time.Duration(2.0*other.Vec[2]*secondsToNanoseconds)
	client.Update(other, rtt)

	// The client should have scooted down to get away from it.
	c = client.GetCoordinate()
	if !(c.Vec[2] < 0.0) {
		t.Fatalf("client z coordinate %9.6f should be < 0.0", c.Vec[2])
	}
}

func TestClient_DistanceTo(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3

	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}

	// Fiddle a raw coordinate to put it a specific number of seconds away.
	other := NewCoordinate(config)
	other.Vec[2] = 12.345
	expected := time.Duration(other.Vec[2]*secondsToNanoseconds)
	dist := client.DistanceTo(other)
	if dist != expected {
		t.Fatalf("distance doesn't match %9.6f != %9.6f", dist.Seconds(), expected.Seconds())
	}
}
