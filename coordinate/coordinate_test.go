package coordinate

import (
	"math"
	"reflect"
	"testing"
)

// verifyEqualFloats will compare f1 and f2 and fail if they are not
// "equal" within a threshold.
func verifyEqualFloats(t *testing.T, f1 float64, f2 float64) {
	const zeroThreshold = 1.0e-6
	if math.Abs(f1 - f2) > zeroThreshold {
		t.Fatalf("equal assertion fail, %9.6f != %9.6f", f1, f2)
	}
}

// verifyEqualVectors will compare vec1 and vec2 and fail if they are not
// "equal" within a threshold.
func verifyEqualVectors(t *testing.T, vec1 []float64, vec2 []float64) {
	if len(vec1) != len(vec2) {
		t.Fatalf("vector length mismatch, %d != %d", len(vec1), len(vec2))
	}

	for i, _ := range vec1 {
		verifyEqualFloats(t, vec1[i], vec2[i])
	}
}

// verifyDimensionPanic will run the supplied func and make sure it panics with
// the expected error type.
func verifyDimensionPanic(t *testing.T, f func()) {
	defer func() {
		if r:= recover(); r != nil {
			if r != ErrDimensionalityConflict {
				t.Fatalf("panic isn't the right type")
			}
		} else {
			t.Fatalf("didn't get expected panic")
		}
	}()
	f()
}

func TestCoordinate_Clone(t *testing.T) {
	c := NewCoordinate(DefaultConfig())
	c.Vec[0], c.Vec[1], c.Vec[2] = 1.0, 2.0, 3.0
	c.Err = 5.0
	c.Adjustment = 10.0

	other := c.Clone()
	if !reflect.DeepEqual(c, other) {
		t.Fatalf("coordinate clone didn't make a proper copy")
	}

	other.Vec[0] = c.Vec[0] + 0.5
	if reflect.DeepEqual(c, other) {
		t.Fatalf("cloned coordinate is still pointing at its ancestor")
	}
}

func TestCoordinate_ApplyForce(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3

	origin := NewCoordinate(config)

	// This proves that we normalize, get the direction right, and apply the
	// force multiplier correctly.
	above := NewCoordinate(config)
	above.Vec = []float64 {0.0, 0.0, 2.9}
	c := origin.ApplyForce(5.3, above)
	verifyEqualVectors(t, c.Vec, []float64 {0.0, 0.0, -5.3})

	// Scoot a point not starting at the origin to make sure there's nothing
	// special there.
	right := NewCoordinate(config)
	right.Vec = []float64 {3.4, 0.0, -5.3}
	c = c.ApplyForce(2.0, right)
	verifyEqualVectors(t, c.Vec, []float64 {-2.0, 0.0, -5.3})

	// If the points are right on top of each other, then we should end up
	// in a random direction, one unit away. This makes sure the unit vector
	// build up doesn't divide by zero.
	c = origin.ApplyForce(1.0, origin)
	verifyEqualFloats(t, origin.DistanceTo(c), 1.0)

	// Shenanigans should get called if the dimensions don't match.
	bad := c.Clone()
	bad.Vec = make([]float64, len(bad.Vec) + 1)
	verifyDimensionPanic(t, func() { c.ApplyForce(1.0, bad) })
}

func TestCoordinate_DistanceTo(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3

	c1, c2 := NewCoordinate(config), NewCoordinate(config)
	c1.Vec = []float64 {-0.5, 1.3, 2.4}
	c2.Vec = []float64 {1.2, -2.3, 3.4}

	verifyEqualFloats(t, c1.DistanceTo(c1), 0.0)
	verifyEqualFloats(t, c1.DistanceTo(c2), c2.DistanceTo(c1))
	verifyEqualFloats(t, c1.DistanceTo(c2), 4.104875150354758)

	// Shenanigans should get called if the dimensions don't match.
	bad := c1.Clone()
	bad.Vec = make([]float64, len(bad.Vec) + 1)
	verifyDimensionPanic(t, func() { _ = c1.DistanceTo(bad) })
}

func TestCoordinate_add(t *testing.T) {
	vec1 := []float64 {1.0, -3.0, 3.0}
	vec2 := []float64 {-4.0, 5.0, 6.0}
	verifyEqualVectors(t, add(vec1, vec2), []float64 {-3.0, 2.0, 9.0})

	zero := []float64 {0.0, 0.0, 0.0}
	verifyEqualVectors(t, add(vec1, zero), vec1)
}

func TestCoordinate_diff(t *testing.T) {
	vec1 := []float64 {1.0, -3.0, 3.0}
	vec2 := []float64 {-4.0, 5.0, 6.0}
	verifyEqualVectors(t, diff(vec1, vec2), []float64 {5.0, -8.0, -3.0})

	zero := []float64 {0.0, 0.0, 0.0}
	verifyEqualVectors(t, diff(vec1, zero), vec1)
}

func TestCoordinate_magnitude(t *testing.T) {
	zero := []float64 {0.0, 0.0, 0.0}
	verifyEqualFloats(t, magnitude(zero), 0.0)

	vec := []float64 {1.0, -2.0, 3.0}
	verifyEqualFloats(t, magnitude(vec), 3.7416573867739413)
}

func TestCoordinate_unitVectorAt(t *testing.T) {
	vec1 := []float64 {1.0, 2.0, 3.0}
	vec2 := []float64 {0.5, 0.6, 0.7}
	u := unitVectorAt(vec1, vec2)
	verifyEqualVectors(t, u, []float64 {0.18257418583505536, 0.511207720338155, 0.8398412548412546})
	verifyEqualFloats(t, magnitude(u), 1.0)

	// If we give positions that are equal we should get a random unit vector
	// returned to us, rather than a divide by zero.
	u = unitVectorAt(vec1, vec1)
	verifyEqualFloats(t, magnitude(u), 1.0)

	// We can't hit the final clause without heroics so I manually forced it
	// there to verify it works.
}
