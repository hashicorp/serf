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
	a.vec[0] = 1
	a.vec[1] = 1
	a.vec[2] = 1

	b := NewCoordinate(config)
	b.vec[0] = 2
	b.vec[1] = 3
	b.vec[2] = 4

	sum, err := a.Add(b, config)
	if err != nil {
		t.Fatal(err)
	}
	sum2, err := b.Add(a, config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sum, sum2) {
		t.Fatalf("addition should be symmetrical")
	}

	if !(sum.vec[0] == 3 && sum.vec[1] == 4 && sum.vec[2] == 5) {
		t.Fatalf("incorrect sum: %+v", sum)
	}

	diff, err := b.Sub(a, config)
	if err != nil {
		t.Fatal(err)
	}
	if !(diff.vec[0] == 1 && diff.vec[1] == 2 && diff.vec[2] == 3) {
		t.Fatalf("incorrect difference: %+v", diff)
	}

	dist, err := a.DistanceTo(b, config)
	if err != nil {
		t.Fatal(err)
	}
	dist2, err := b.DistanceTo(a, config)
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
		a.Update(b.coord, rtt)
		b.Update(a.coord, rtt)
	}

	dist, err := a.DistanceTo(b.coord)
	if err != nil {
		t.Fatal(err)
	}
	if !(math.Abs(float64((rtt - dist).Nanoseconds())) < 0.01*float64(rtt.Nanoseconds())) {
		t.Fatalf("The computed distance should be %f but is actually %f.\n%+v\n%+v",
			rtt, dist, a, b)
	}
}
