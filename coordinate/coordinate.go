package coordinate

import (
	"fmt"
	"math"
	"math/rand"
)

const (
	HEIGHT_THRESHOLD = 0.01
)

// Coordinate is a Vivaldi network coordinate.  Refer to the Vivaldi paper for a detailed
// description.
type Coordinate struct {
	Vec    []float64
	Height float64
}

// NewCoordinate creates a new network coordinate located at the origin
func NewCoordinate(dimension uint) *Coordinate {
	return &Coordinate{
		Vec:    make([]float64, dimension),
		Height: HEIGHT_THRESHOLD,
	}
}

// Add is used to add a given coordinate to the receiver, returning the new coordinate
func (self *Coordinate) Add(other *Coordinate) (*Coordinate, error) {
	if len(self.Vec) != len(other.Vec) {
		return nil, fmt.Errorf("adding two coordinates that have different dimensions:\n%+v\n%+v", self, other)
	} else {
		ret := NewCoordinate(uint(len(self.Vec)))

		if ret.Height < HEIGHT_THRESHOLD {
			ret.Height = HEIGHT_THRESHOLD
		}

		for i, _ := range self.Vec {
			ret.Vec[i] = self.Vec[i] + other.Vec[i]
		}

		return ret, nil
	}
}

// Sub is used to subtract a given coordinate from the receiver, returning the new coordinate
func (self *Coordinate) Sub(other *Coordinate) (*Coordinate, error) {
	if len(self.Vec) != len(other.Vec) {
		return nil, fmt.Errorf("subtracting two coordinates that have different dimensions:\n%+v\n%+v", self, other)
	} else {
		ret := NewCoordinate(uint(len(self.Vec)))

		ret.Height = self.Height + other.Height

		for i, _ := range self.Vec {
			ret.Vec[i] = self.Vec[i] - other.Vec[i]
		}

		return ret, nil
	}
}

// Mul is used to multiple a given factor with the receiver, returning the new coordinate
func (self *Coordinate) Mul(factor float64) *Coordinate {
	ret := NewCoordinate(uint(len(self.Vec)))

	ret.Height = self.Height * float64(factor)
	if ret.Height < HEIGHT_THRESHOLD {
		ret.Height = HEIGHT_THRESHOLD
	}

	for i, _ := range self.Vec {
		ret.Vec[i] = self.Vec[i] * float64(factor)
	}

	return ret
}

// DistanceTo returns the distance between the given coordinate and the receiver
func (self *Coordinate) DistanceTo(other *Coordinate) (float64, error) {
	tmp, err := self.Sub(other)
	if err != nil {
		return 0, err
	}

	sum := 0.0
	for i, _ := range self.Vec {
		sum += math.Pow(tmp.Vec[i], 2)
	}

	return math.Sqrt(sum) + tmp.Height, nil
}

// DirectionTo returns a coordinate that represents a unit-length vector, which represents
// the direction from the receiver to the given coordinate.  In case the two coordinates are
// located together, a random direction is returned.
func (self *Coordinate) DirectionTo(other *Coordinate) (*Coordinate, error) {
	tmp, err := self.Sub(other)
	if err != nil {
		return nil, err
	}

	dist, err := self.DistanceTo(other)
	if err != nil {
		return nil, err
	}

	if dist != self.Height+other.Height {
		tmp = tmp.Mul(1.0 / dist)
		return tmp, nil
	} else {
		for i, _ := range self.Vec {
			tmp.Vec[i] = (10-0.1)*rand.Float64() + 0.1
		}
		dist, err = tmp.DistanceTo(NewCoordinate(uint(len(self.Vec))))
		if err != nil {
			return nil, err
		}

		tmp = tmp.Mul(1.0 / dist)
		return tmp, nil
	}
}
