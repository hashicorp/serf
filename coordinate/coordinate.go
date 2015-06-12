package coordinate

import (
	"errors"
	"math"
	"math/rand"
)

// Coordinate is a specialized structure for holding network coordinates for the
// Vivaldi-based coordinate mapping algorithm. All of the fields should be public
// to enable this to be serialized.
type Coordinate struct {
	// Vec is the Euclidian portion of the coordinate. This is used along
	// with the other fields to provide an overall distance estimate.
	Vec        []float64

	// Err reflects the confidence in the given coordinate and is updated
	// dynamically by the Vivaldi Client.
	Err        float64

	// Adjustment is a distance offset computed based on a calculation over
	// observations from all other nodes over a fixed window and is updated
	// dynamically by the Vivaldi Client.
	Adjustment float64
}

var (
	// ErrDimensionalityConflict will be panic-d if you try to perform
	// operations with incompatible dimensions.
	ErrDimensionalityConflict = errors.New("coordinate dimensionality does not match")
)

// NewCoordinate creates a new coordinate at the origin, using the given config
// to supply key initial values.
func NewCoordinate(config *Config) (*Coordinate) {
	return &Coordinate{
		Vec:        make([]float64, config.Dimensionality),
		Err:        config.MaxVivaldiError,
		Adjustment: 0.0,
	}
}

// Clone creates an independent copy of this coordinate.
func (c *Coordinate) Clone() *Coordinate {
	vec := make([]float64, len(c.Vec))
	copy(vec, c.Vec)
	return &Coordinate{
		Vec:        vec,
		Err:        c.Err,
		Adjustment: c.Adjustment,
	}
}

// ApplyForce returns the result of applying the force in the direction of the
// other coordinate.
func (c *Coordinate) ApplyForce(force float64, other *Coordinate) (*Coordinate) {
	if len(c.Vec) != len(other.Vec) {
		panic(ErrDimensionalityConflict)
	}

	ret := c.Clone()
	ret.Vec = add(ret.Vec, mul(unitVectorAt(ret.Vec, other.Vec), force))
	return ret
}

// DistanceTo returns the distance between this coordinate and the other
// coordinate.
func (c *Coordinate) DistanceTo(other *Coordinate) (float64) {
	if len(c.Vec) != len(other.Vec) {
		panic(ErrDimensionalityConflict)
	}

	euclidianPart := magnitude(diff(c.Vec, other.Vec))
	adjustmentPart := c.Adjustment + other.Adjustment
	return euclidianPart + adjustmentPart
}

// add returns the sum of vec1 and vec2. This assumes the dimensions have
// already been checked to be compatible.
func add(vec1 []float64, vec2 []float64) ([]float64) {
	ret := make([]float64, len(vec1))
	for i, _ := range ret {
		ret[i] = vec1[i] + vec2[i]
	}
	return ret
}

// diff returns the difference between the vec1 and vec2. This assumes the
// dimensions have already been checked to be compatible.
func diff(vec1 []float64, vec2 []float64) ([]float64) {
	ret := make([]float64, len(vec1))
	for i, _ := range ret {
		ret[i] = vec1[i] - vec2[i]
	}
	return ret
}

// mul returns vec multiplied by a scalar factor.
func mul(vec []float64, factor float64) ([]float64) {
	ret := make([]float64, len(vec))
	for i, _ := range vec {
		ret[i] = vec[i] * factor
	}
	return ret
}

// magnitude computes the magnitude of the vec.
func magnitude(vec []float64) (float64) {
	sum := 0.0
	for i, _ := range vec {
		sum += vec[i] * vec[i]
	}
	return math.Sqrt(sum)
}

// unitVectorAt returns a unit vector pointing at vec1 from vec2 (the way an
// object positioned at vec1 would move if it was being repelled by an object at
// vec2). If the two positions are the same then a random unit vector is returned.
func unitVectorAt(vec1 []float64, vec2 []float64) ([]float64) {
	ret := diff(vec1, vec2)

	// If the coordinates aren't on top of each other we can normalize.
	const zeroThreshold = 1.0e-6
	if mag := magnitude(ret); mag > zeroThreshold {
		return mul(ret, 1.0/mag)
	}

	// Otherwise, just return a random unit vector.
	for i, _ := range ret {
		ret[i] = rand.Float64() - 0.5
	}
	if mag := magnitude(ret); mag > zeroThreshold {
		return mul(ret, 1.0/mag)
	}

	// And finally just give up and make a unit vector along the first
	// dimension. This should be exceedingly rare.
	for i, _ := range ret {
		if i == 0 {
			ret[i] = 1.0
		} else {
			ret[i] = 0.0
		}
	}
	return ret
}
