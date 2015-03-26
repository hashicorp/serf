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
	Vec        []float64
	Height     float64
	Err        float64
	Adjustment float64
}

// NewCoordinate creates a new network coordinate located at the origin
func NewCoordinate(config *ClientConfig) *Coordinate {
	return &Coordinate{
		Vec:        make([]float64, config.Dimension),
		Height:     config.HeightThreshold,
		Err:        config.VivaldiError,
		Adjustment: 0,
	}
}

// Add is used to add up two coordinates, returning the sum
func (self *Client) Add(this, that *Coordinate) (*Coordinate, error) {
	if len(this.Vec) != len(that.Vec) {
		return nil, fmt.Errorf("adding two coordinates that have different dimensions:\n%+v\n%+v", this, that)
	} else {
		ret := NewCoordinate(self.config)

		if ret.Height < self.config.HeightThreshold {
			ret.Height = self.config.HeightThreshold
		}

		for i, _ := range this.Vec {
			ret.Vec[i] = this.Vec[i] + that.Vec[i]
		}

		return ret, nil
	}
}

// Sub is used to subtract the second coordinate from the first, returning the diff
func (self *Client) Sub(this, that *Coordinate) (*Coordinate, error) {
	if len(this.Vec) != len(that.Vec) {
		return nil, fmt.Errorf("subtracting two coordinates that have different dimensions:\n%+v\n%+v", this, that)
	} else {
		ret := NewCoordinate(self.config)

		ret.Height = this.Height + that.Height

		for i, _ := range this.Vec {
			ret.Vec[i] = this.Vec[i] - that.Vec[i]
		}

		return ret, nil
	}
}

// Mul is used to multiply a given factor with the given coordinate, returning a new coordinate
func (self *Client) Mul(coord *Coordinate, factor float64) *Coordinate {
	ret := NewCoordinate(self.config)

	ret.Height = coord.Height * float64(factor)
	if ret.Height < self.config.HeightThreshold {
		ret.Height = self.config.HeightThreshold
	}

	for i, _ := range coord.Vec {
		ret.Vec[i] = coord.Vec[i] * float64(factor)
	}

	return ret
}

// DistanceBetween returns the distance between the two given coordinates
func (self *Client) DistanceBetween(this, that *Coordinate) (float64, error) {
	tmp, err := self.Sub(this, that)
	if err != nil {
		return 0, err
	}

	sum := 0.0
	for i, _ := range tmp.Vec {
		sum += math.Pow(tmp.Vec[i], 2)
	}

	return math.Sqrt(sum) + tmp.Height, nil
}

// DirectionBetween returns a coordinate that represents a unit-length vector, which represents
// the direction from the first coordinate to the second.  In case the two coordinates are
// located together, a random direction is returned.
func (self *Client) DirectionBetween(this, that *Coordinate) (*Coordinate, error) {
	tmp, err := self.Sub(this, that)
	if err != nil {
		return nil, err
	}

	dist, err := self.DistanceBetween(this, that)
	if err != nil {
		return nil, err
	}

	if dist != this.Height+that.Height {
		tmp = self.Mul(tmp, 1.0/dist)
		return tmp, nil
	} else {
		for i, _ := range this.Vec {
			tmp.Vec[i] = (10-0.1)*rand.Float64() + 0.1
		}
		dist, err = self.DistanceBetween(tmp, NewCoordinate(self.config))
		if err != nil {
			return nil, err
		}

		tmp = self.Mul(tmp, 1.0/dist)
		return tmp, nil
	}
}
