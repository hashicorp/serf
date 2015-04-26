package coordinate

import (
	"math"
	"reflect"
	"testing"
)

func floatingPointEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.01*math.Abs(a)
}

// Return two constant coordinates and the config used to create them
// A is at (1, 1, 1),  while B is at (2, 3, 4).
// The math I have done shows that:
//	A + B = (3, 4, 5)
//	B - A = (1, 2, 3)
//  dist(A, B) = sqrt(14)
func getTwoConstantCoordinates() (a, b *Coordinate, config *Config) {
	config = DefaultConfig()
	config.Dimension = 3

	a = NewCoordinate(config)
	a.Vec[0] = 1
	a.Vec[1] = 1
	a.Vec[2] = 1

	b = NewCoordinate(config)
	b.Vec[0] = 2
	b.Vec[1] = 3
	b.Vec[2] = 4

	return
}

func TestCoordinateAdd(t *testing.T) {
	a, b, config := getTwoConstantCoordinates()

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

	if !(sum.Vec[0] == 3 && sum.Vec[1] == 4 && sum.Vec[2] == 5) {
		t.Fatalf("incorrect sum: %+v", sum)
	}
}

func TestCoordinateSub(t *testing.T) {
	a, b, config := getTwoConstantCoordinates()

	diff, err := b.Sub(a, config)
	if err != nil {
		t.Fatal(err)
	}
	if !(diff.Vec[0] == 1 && diff.Vec[1] == 2 && diff.Vec[2] == 3) {
		t.Fatalf("incorrect difference: %+v", diff)
	}
}

func TestCoordinateDistanceTo(t *testing.T) {
	a, b, config := getTwoConstantCoordinates()

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

func TestCoordinateDirectionTo(t *testing.T) {
	a, b, config := getTwoConstantCoordinates()
	origin := NewCoordinate(config)

	atob, err := a.DirectionTo(b, config)
	if err != nil {
		t.Fatal(err)
	}
	btoa, err := b.DirectionTo(a, config)
	if err != nil {
		t.Fatal(err)
	}

	atobDist, err := atob.DistanceTo(origin, config)
	if err != nil {
		t.Fatal(err)
	}
	btoaDist, err := btoa.DistanceTo(origin, config)
	if err != nil {
		t.Fatal(err)
	}

	if !(floatingPointEqual(atobDist, btoaDist)) {
		t.Fatalf("Opposite direction vectors between the same two points should be of the same length: %v %v", atobDist, btoaDist)
	}

	if !(floatingPointEqual(atobDist, 1+atob.Height)) {
		t.Fatalf("Direction vectors should be unit-length")
	}
}
