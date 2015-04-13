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
	vec        []float64
	height     float64
	err        float64
	adjustment float64
}

// NewCoordinate creates a new network coordinate located at the origin
func NewCoordinate(config *Config) *Coordinate {
	return &Coordinate{
		vec:        make([]float64, config.Dimension),
		height:     config.HeightThreshold,
		err:        config.VivaldiError,
		adjustment: 0,
	}
}

// Add is used to add up two coordinates, returning the sum
func (c *Coordinate) Add(other *Coordinate, conf *Config) (*Coordinate, error) {
	if len(c.vec) != len(other.vec) {
		return nil, fmt.Errorf("adding two coordinates that have different dimensions:\n%+v\n%+v", c, other)
	} else {
		ret := NewCoordinate(conf)

		if ret.height < conf.HeightThreshold {
			ret.height = conf.HeightThreshold
		}

		for i, _ := range c.vec {
			ret.vec[i] = c.vec[i] + other.vec[i]
		}

		return ret, nil
	}
}

// Sub is used to subtract the second coordinate from the first, returning the diff
func (c *Coordinate) Sub(other *Coordinate, conf *Config) (*Coordinate, error) {
	if len(c.vec) != len(other.vec) {
		return nil, fmt.Errorf("subtracting two coordinates that have different dimensions:\n%+v\n%+v", c, other)
	} else {
		ret := NewCoordinate(conf)

		ret.height = c.height + other.height

		for i, _ := range c.vec {
			ret.vec[i] = c.vec[i] - other.vec[i]
		}

		return ret, nil
	}
}

// Mul is used to multiply a given factor with the given coordinate, returning a new coordinate
func (c *Coordinate) Mul(factor float64, conf *Config) *Coordinate {
	ret := NewCoordinate(conf)

	ret.height = c.height * float64(factor)
	if ret.height < conf.HeightThreshold {
		ret.height = conf.HeightThreshold
	}

	for i, _ := range c.vec {
		ret.vec[i] = c.vec[i] * float64(factor)
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
	for i, _ := range tmp.vec {
		sum += math.Pow(tmp.vec[i], 2)
	}

	return math.Sqrt(sum) + tmp.height, nil
}

// DirectionTo returns a coordinate other represents a unit-length vector, which represents
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

	if dist != c.height+coord.height {
		tmp = tmp.Mul(1.0/dist, conf)
		return tmp, nil
	} else {
		for i, _ := range c.vec {
			tmp.vec[i] = (10-0.1)*rand.Float64() + 0.1
		}
		dist, err = tmp.DistanceTo(NewCoordinate(conf), conf)
		if err != nil {
			return nil, err
		}

		tmp = tmp.Mul(1.0/dist, conf)
		return tmp, nil
	}
}
