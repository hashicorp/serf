package coordinate

import (
	"math"
	"time"
)

const (
	// The dimension of the coordinate system.  The paper "Network Coordinates in the Wild" has shown
	// that the accuracy of a coordinate system increases with the number of dimensions, but only up
	// to a certain point.  Specifically, there is no noticeable improvement beyond 7 dimensions.  Here
	// we use 8.
	DIMENTION = 8

	// The following are the constants used in the computation of Vivaldi coordinates.  For a detailed
	// description of what each of them means, please refer to the Vivaldi paper.
	VIVALDI_ERROR             = 1.5
	VIVALDI_ERROR_UNCONNECTED = 2
	VIVALDI_CE                = 0.25
	VIVALDI_CC                = 0.25

	// The number of measurements we use to update the adjustment term.
	// Instead of using a constant, we should probably dynamically adjust this
	// using the cluster size and the gossip rate.
	ADJUSTMENT_WINDOW_SIZE = 10
)

// Client consists of a network coordinate, an error estimation, and an adjustment term.  All three
// elements are needed to compute network distance.
type Client struct {
	Coord *Coordinate
	Err   float64
	// The unit of time used for the following fields is millisecond
	Adjustment        float64
	adjustment_index  uint      // index into adjustment window
	adjustment_window []float64 // a rolling window that stores the differences between expected distances and real distances
}

// NewClient creates a new Client.
func NewClient() *Client {
	return &Client{
		Coord:             NewCoordinate(DIMENTION),
		Err:               VIVALDI_ERROR,
		Adjustment:        0,
		adjustment_index:  0,
		adjustment_window: make([]float64, ADJUSTMENT_WINDOW_SIZE),
	}
}

// Update takes a Client, which contains the position of another node, and the rtt between the receiver
// and the other node, and updates the position of the receiver.
func (self *Client) Update(other *Client, rtt_dur time.Duration) error {
	rtt := float64(rtt_dur.Nanoseconds()) / (1000 * 1000) // 1 millisecond = 1000 * 1000 nanoseconds
	dist, err := self.Coord.DistanceTo(other.Coord)
	if err != nil {
		return err
	}

	weight := self.Err / (self.Err + other.Err)
	err_calc := math.Abs(dist-rtt) / rtt
	self.Err = err_calc*VIVALDI_CE*weight + self.Err*(1-VIVALDI_CE*weight)
	if self.Err > VIVALDI_ERROR {
		self.Err = VIVALDI_ERROR
	}
	delta := VIVALDI_CC * weight

	direction, err := self.Coord.DirectionTo(other.Coord)
	if err != nil {
		return err
	}

	self.Coord, err = self.Coord.Add(direction.Mul(delta * (rtt - dist)))
	if err != nil {
		return err
	}

	self.updateAdjustment(other, rtt)
	return nil
}

func (self *Client) updateAdjustment(other *Client, rtt float64) error {
	dist, err := self.Coord.DistanceTo(other.Coord)
	if err != nil {
		return err
	}
	self.adjustment_window[self.adjustment_index] = rtt - dist
	self.adjustment_index = (self.adjustment_index + 1) % ADJUSTMENT_WINDOW_SIZE
	tmp := 0.0
	for _, n := range self.adjustment_window {
		tmp += n
	}
	self.Adjustment = tmp / (2.0 * ADJUSTMENT_WINDOW_SIZE)
	return nil
}

// DistanceTo takes a Client, which contains the position of another node, and computes the distance
// between the receiver and the other node.
func (self *Client) DistanceTo(other *Client) (time.Duration, error) {
	dist, err := self.Coord.DistanceTo(other.Coord)
	if err != nil {
		return time.Duration(0), err
	}
	return time.Duration(dist+self.Adjustment+other.Adjustment) * time.Millisecond, nil
}
