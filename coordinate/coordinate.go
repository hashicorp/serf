package coordinate

import (
	"fmt"
	"math"
	"math/rand"
)

// Coordinate is a Vivaldi network coordinate.  Refer to the Vivaldi paper for a detailed
// description.
type Coordinate struct {
	// The unit of time used for the following fields is millisecond
	// The fields need to be public for them to be serializable
	Vec        []float64
	Height     float64
	Err        float64
	Adjustment float64
}

// NewCoordinate creates a new network coordinate located at the origin
func NewCoordinate(config *Config) *Coordinate {
	return &Coordinate{
		Vec:        make([]float64, config.Dimension),
		Height:     config.HeightThreshold,
		Err:        config.VivaldiError,
		Adjustment: 0,
	}
}

// Add is used to add up two coordinates, returning the sum
func (c *Coordinate) Add(other *Coordinate, conf *Config) (*Coordinate, error) {
	if len(c.Vec) != len(other.Vec) {
		return nil, fmt.Errorf("adding two coordinates that have different dimensions:\n%+v\n%+v", c, other)
	} else {
		ret := NewCoordinate(conf)

		if ret.Height < conf.HeightThreshold {
			ret.Height = conf.HeightThreshold
		}

		for i, _ := range c.Vec {
			ret.Vec[i] = c.Vec[i] + other.Vec[i]
		}

		return ret, nil
	}
}

// Sub is used to subtract the second coordinate from the first, returning the diff
func (c *Coordinate) Sub(other *Coordinate, conf *Config) (*Coordinate, error) {
	if len(c.Vec) != len(other.Vec) {
		return nil, fmt.Errorf("subtracting two coordinates that have different dimensions:\n%+v\n%+v", c, other)
	} else {
		ret := NewCoordinate(conf)

		ret.Height = c.Height + other.Height

		for i, _ := range c.Vec {
			ret.Vec[i] = c.Vec[i] - other.Vec[i]
		}

		return ret, nil
	}
}

// Mul is used to multiply a given factor with the given coordinate, returning a new coordinate
func (c *Coordinate) Mul(factor float64, conf *Config) *Coordinate {
	ret := NewCoordinate(conf)

	ret.Height = c.Height * float64(factor)
	if ret.Height < conf.HeightThreshold {
		ret.Height = conf.HeightThreshold
	}

	for i, _ := range c.Vec {
		ret.Vec[i] = c.Vec[i] * float64(factor)
	}

	return ret
}

// DistanceTo returns the distance between the receiver and the given coordinate
func (c *Coordinate) DistanceTo(coord *Coordinate, conf *Config) (float64, error) {
	tmp, err := c.Sub(coord, conf)
	if err != nil {
		return 0, err
	}

	sum := 0.0
	for i, _ := range tmp.Vec {
		sum += math.Pow(tmp.Vec[i], 2)
	}

	return math.Sqrt(sum) + tmp.Height, nil
}

// DirectionTo returns a coordinate other represents a unit-length Vector, which represents
// the direction from the receiver to the given coordinate.  In case the two coordinates are
// located together, a random direction is returned.
func (c *Coordinate) DirectionTo(coord *Coordinate, conf *Config) (*Coordinate, error) {
	tmp, err := c.Sub(coord, conf)
	if err != nil {
		return nil, err
	}

	dist, err := c.DistanceTo(coord, conf)
	if err != nil {
		return nil, err
	}

	if dist != c.Height+coord.Height {
		tmp = tmp.Mul(1.0/dist, conf)
		return tmp, nil
	} else {
		for i, _ := range c.Vec {
			tmp.Vec[i] = (10-0.1)*rand.Float64() + 0.1
		}
		dist, err = tmp.DistanceTo(NewCoordinate(conf), conf)
		if err != nil {
			return nil, err
		}

		tmp = tmp.Mul(1.0/dist, conf)
		return tmp, nil
	}
}
