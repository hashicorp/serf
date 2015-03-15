package coordinate

import (
	"math"
	"reflect"
	"testing"
)

func TestCoordinate(t *testing.T) {
	// We have two points A and B in a 3-d space.  A is at (1, 1, 1),  while B is at (2, 3, 4).
	// The math I have done shows that:
	//	A + B = (3, 4, 5)
	//	B - A = (1, 2, 3)
	//  dist(A, B) = sqrt(14)
	dim := uint(3)

	a := NewCoordinate(dim)
	a.vec[0] = 1
	a.vec[1] = 1
	a.vec[2] = 1

	b := NewCoordinate(dim)
	b.vec[0] = 2
	b.vec[1] = 3
	b.vec[2] = 4

	sum := a.Add(b)
	if !reflect.DeepEqual(sum, b.Add(a)) {
		t.Fatalf("addition should be symmetrical")
	}

	if !(sum.vec[0] == 3 && sum.vec[1] == 4 && sum.vec[2] == 5) {
		t.Fatalf("incorrect sum: %+v", sum)
	}

	diff := b.Sub(a)
	if !(diff.vec[0] == 1 && diff.vec[1] == 2 && diff.vec[2] == 3) {
		t.Fatalf("incorrect difference: %+v", diff)
	}

	dist := a.DistanceTo(b)
	if !(dist == b.DistanceTo(a) && math.Abs(dist-math.Sqrt(14)) < 0.01*dist) {
		t.Fatalf("incorrect distance: %f", dist)
	}
}

func TestAlgorithm(t *testing.T) {
	rtt := 100.0
	a := NewClient()
	b := NewClient()
	for i := 0; i < 100000; i++ {
		a.Update(&b, rtt)
		b.Update(&a, rtt)
	}

	if !(math.Abs(rtt-a.DistanceTo(&b)) < 0.01*rtt) {
		t.Fatalf("The computed distance should be %f but is actually %f.\n%+v\n%+v",
			rtt, a.DistanceTo(&b), a, b)
	}
}
