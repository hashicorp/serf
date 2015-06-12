package coordinate

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	config := DefaultConfig()
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}

	origin := NewCoordinate(config)
	if !reflect.DeepEqual(origin, client.GetCoordinate()) {
		t.Fatalf("A new client should come with a new coordinate")
	}
}

func TestUpdate(t *testing.T) {
	rtt := 100.0 * time.Millisecond
	a, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100000; i++ {
		err := a.Update(b.coord, rtt)
		if err != nil {
			t.Fatal(err)
		}
		err = b.Update(a.coord, rtt)
		if err != nil {
			t.Fatal(err)
		}
	}

	dist := a.DistanceTo(b.coord)
	if !(math.Abs(float64((rtt - dist).Nanoseconds())) < 0.01*float64(rtt.Nanoseconds())) {
		t.Fatalf("The computed distance should be %f but is actually %f.\n%+v\n%+v",
			rtt, dist, a, b)
	}
}

/*

func TestUpdateError(t *testing.T) {
	config1 := DefaultConfig()
	config2 := DefaultConfig()
	config2.Dimensionality += 1

	client, err := NewClient(config1)
	if err != nil {
		t.Fatal(err)
	}
	coord := NewCoordinate(config2)
	err = client.Update(coord, time.Second)
	if err == nil {
		t.Fatalf("Updating using a coord with the wrong dimensionality should result in an error")
	}
}

*/
