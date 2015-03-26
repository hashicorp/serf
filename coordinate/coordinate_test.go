package coordinate

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestCoordinate(t *testing.T) {
	// We have two points A and B in a 3-d space.  A is at (1, 1, 1),  while B is at (2, 3, 4).
	// The math I have done shows that:
	//	A + B = (3, 4, 5)
	//	B - A = (1, 2, 3)
	//  dist(A, B) = sqrt(14)
	config := DefaultConfig()
	config.Dimension = 3

	a := NewCoordinate(config)
	a.Vec[0] = 1
	a.Vec[1] = 1
	a.Vec[2] = 1

	b := NewCoordinate(config)
	b.Vec[0] = 2
	b.Vec[1] = 3
	b.Vec[2] = 4

	client := NewClient(config)

	sum, err := client.Add(a, b)
	if err != nil {
		t.Fatal(err)
	}
	sum2, err := client.Add(b, a)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sum, sum2) {
		t.Fatalf("addition should be symmetrical")
	}

	if !(sum.Vec[0] == 3 && sum.Vec[1] == 4 && sum.Vec[2] == 5) {
		t.Fatalf("incorrect sum: %+v", sum)
	}

	diff, err := client.Sub(b, a)
	if err != nil {
		t.Fatal(err)
	}
	if !(diff.Vec[0] == 1 && diff.Vec[1] == 2 && diff.Vec[2] == 3) {
		t.Fatalf("incorrect difference: %+v", diff)
	}

	dist, err := client.DistanceBetween(a, b)
	if err != nil {
		t.Fatal(err)
	}
	dist2, err := client.DistanceBetween(b, a)
	if err != nil {
		t.Fatal(err)
	}
	if !(dist == dist2 && math.Abs(dist-math.Sqrt(14)) < 0.01*dist) {
		t.Fatalf("incorrect distance: %f", dist)
	}
}

func TestAlgorithm(t *testing.T) {
	rtt := 100.0 * time.Millisecond
	a := NewClient(DefaultConfig())
	b := NewClient(DefaultConfig())
	for i := 0; i < 100000; i++ {
		a.Update(b.Coord, rtt)
		b.Update(a.Coord, rtt)
	}

	dist, err := a.DistanceTo(b.Coord)
	if err != nil {
		t.Fatal(err)
	}
	if !(math.Abs(float64((rtt - dist).Nanoseconds())) < 0.01*float64(rtt.Nanoseconds())) {
		t.Fatalf("The computed distance should be %f but is actually %f.\n%+v\n%+v",
			rtt, dist, a, b)
	}
}
