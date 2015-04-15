package coordinate

import (
	"math"
	"sync"
	"time"
)

// Config is used to provide specific parameters to the Vivaldi algorithm
type Config struct {
	// The dimension of the coordinate system.  The paper "Network Coordinates in the Wild" has shown
	// that the accuracy of a coordinate system increases with the number of dimensions, but only up
	// to a certain point.  Specifically, there is no noticeable improvement beyond 7 dimensions.
	Dimension uint

	// The following are the constants used in the computation of Vivaldi coordinates.  For a detailed
	// description of what each of them means, please refer to the Vivaldi paper.
	VivaldiError    float64
	VivaldiCE       float64
	VivaldiCC       float64
	HeightThreshold float64

	// The number of measurements we use to update the adjustment term.
	// Instead of using a constant, we should probably dynamically adjust this
	// using the cluster size and the gossip rate.
	AdjustmentWindowSize uint
}

// DefaultConfig returns a Config that has the default values
func DefaultConfig() *Config {
	return &Config{
		Dimension:            8,
		VivaldiError:         1.5,
		VivaldiCE:            0.25,
		VivaldiCC:            0.25,
		HeightThreshold:      0.01,
		AdjustmentWindowSize: 10,
	}
}

// Client consists of a network coordinate, an error estimation, and an adjustment term.  All three
// elements are needed to compute network distance.
//
// The APIs are all thread-safe.
type Client struct {
	coord             *Coordinate
	adjustment_index  uint      // index into adjustment window
	adjustment_window []float64 // a rolling window that stores the differences between expected distances and real distances
	config            *Config
	mutex             *sync.RWMutex // enable safe conccurent access
}

// NewClient creates a new Client.
func NewClient(config *Config) *Client {
	return &Client{
		coord:             NewCoordinate(config),
		config:            config,
		adjustment_index:  0,
		adjustment_window: make([]float64, config.AdjustmentWindowSize),
		mutex:             &sync.RWMutex{},
	}
}

// GetCoordinate returns the coordinate of this client
func (c *Client) GetCoordinate() *Coordinate {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.coord
}

// Update takes a Client, which contains the position of another node, and the rtt between the receiver
// and the other node, and updates the position of the receiver.
func (c *Client) Update(coord *Coordinate, rttDur time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	rtt := float64(rttDur.Nanoseconds()) / (1000 * 1000) // 1 millisecond = 1000 * 1000 nanoseconds
	dist, err := c.coord.DistanceTo(coord, c.config)
	if err != nil {
		return err
	}

	weight := c.coord.Err / (c.coord.Err + coord.Err)
	err_calc := math.Abs(dist-rtt) / rtt
	c.coord.Err = err_calc*c.config.VivaldiCE*weight + c.coord.Err*(1-c.config.VivaldiCE*weight)
	if c.coord.Err > c.config.VivaldiError {
		c.coord.Err = c.config.VivaldiError
	}
	delta := c.config.VivaldiCC * weight

	direction, err := c.coord.DirectionTo(coord, c.config)
	if err != nil {
		return err
	}

	c.coord, err = c.coord.Add(direction.Mul(delta*(rtt-dist), c.config), c.config)
	if err != nil {
		return err
	}

	c.updateAdjustment(coord, rtt)
	return nil
}

func (c *Client) updateAdjustment(coord *Coordinate, rtt float64) error {
	dist, err := c.coord.DistanceTo(coord, c.config)
	if err != nil {
		return err
	}
	c.adjustment_window[c.adjustment_index] = rtt - dist
	c.adjustment_index = (c.adjustment_index + 1) % c.config.AdjustmentWindowSize
	tmp := 0.0
	for _, n := range c.adjustment_window {
		tmp += n
	}
	c.coord.Adjustment = tmp / (2.0 * float64(c.config.AdjustmentWindowSize))
	return nil
}

// DistanceTo takes a Client, which contains the position of another node, and computes the distance
// between the receiver and the other node.
func (c *Client) DistanceTo(coord *Coordinate) (time.Duration, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	dist, err := c.coord.DistanceTo(coord, c.config)
	if err != nil {
		return time.Duration(0), err
	}
	return time.Duration(dist+c.coord.Adjustment+coord.Adjustment) * time.Millisecond, nil
}
