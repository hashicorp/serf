package coordinate

import (
	"math"
	"time"
)

const (
	DIMENTION                 = 8
	VIVALDI_ERROR             = 1.5
	VIVALDI_ERROR_UNCONNECTED = 2
	VIVALDI_CE                = 0.25
	VIVALDI_CC                = 0.25
	// The number of measurements we use to update the adjustment term.
	// Instead of using a constant, we should probably dynamically adjust this
	// using the cluster size and the gossip rate.
	ADJUSTMENT_WINDOW_SIZE = 10
)

type Client struct {
	Coord Coordinate
	Err   float64
	// The unit of time used for the following fields is millisecond
	Adjustment        float64
	adjustment_index  uint      // index into adjustment window
	adjustment_window []float64 // a rolling window that stores the differences between expected distances and real distances
}

func NewClient() *Client {
	return &Client{
		Coord:             NewCoordinate(DIMENTION),
		Err:               VIVALDI_ERROR,
		Adjustment:        0,
		adjustment_index:  0,
		adjustment_window: make([]float64, ADJUSTMENT_WINDOW_SIZE),
	}
}

func (self *Client) Update(other *Client, _rtt time.Duration) {
	rtt := float64(_rtt.Nanoseconds()) / (1000 * 1000) // 1 millisecond = 1000 * 1000 nanoseconds
	dist := self.Coord.DistanceTo(other.Coord)
	weight := self.Err / (self.Err + other.Err)
	err_calc := math.Abs(dist-rtt) / rtt
	self.Err = err_calc*VIVALDI_CE*weight + self.Err*(1-VIVALDI_CE*weight)
	if self.Err > VIVALDI_ERROR {
		self.Err = VIVALDI_ERROR
	}
	delta := VIVALDI_CC * weight
	self.Coord = self.Coord.Add(self.Coord.DirectionTo(other.Coord).Mul(delta * (rtt - dist)))
	self.updateAdjustment(other, rtt)
}

func (self *Client) updateAdjustment(other *Client, rtt float64) {
	self.adjustment_window[self.adjustment_index] = rtt - self.Coord.DistanceTo(other.Coord)
	self.adjustment_index = (self.adjustment_index + 1) % ADJUSTMENT_WINDOW_SIZE
	tmp := 0.0
	for _, n := range self.adjustment_window {
		tmp += n
	}
	self.Adjustment = tmp / (2.0 * ADJUSTMENT_WINDOW_SIZE)
}

func (self *Client) DistanceTo(other *Client) time.Duration {
	return time.Duration(self.Coord.DistanceTo(other.Coord)+self.Adjustment+other.Adjustment) * time.Millisecond
}
