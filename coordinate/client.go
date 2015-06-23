package coordinate

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Client manages the estimated network coordinate for a given node, and adjusts
// it as the node observes round trip times and estimated coordinates from other
// nodes. The core algorithm is based on Vivaldi, see the documentation for Config
// for more details.
type Client struct {
	// coord is the current estimate of the client's network coordinate.
	coord *Coordinate

	// config contains the tuning parameters that govern the performance of
	// the algorithm.
	config *Config

	// adjustmentIndex is the current index into the adjustmentSamples slice.
	adjustmentIndex uint

	// adjustment is used to store samples for the adjustment calculation.
	adjustmentSamples []float64

	// mutex enables safe concurrent access to the client.
	mutex *sync.RWMutex
}

// NewClient creates a new Client and verifies the configuration is valid.
func NewClient(config *Config) (*Client, error) {
	if !(config.Dimensionality > 0) {
		return nil, fmt.Errorf("dimensionality must be >0")
	}

	return &Client{
		coord:  NewCoordinate(config),
		config: config,
		adjustmentIndex: 0,
		adjustmentSamples: make([]float64, config.AdjustmentWindowSize),
		mutex:  &sync.RWMutex{},
	}, nil
}

// GetCoordinate returns a copy of the coordinate for this client.
func (c *Client) GetCoordinate() *Coordinate {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.coord.Clone()
}

// updateVivialdi updates the Vivaldi portion of the client's coordinate. This
// assumes that the mutex has been locked already.
func (c *Client) updateVivaldi(other *Coordinate, rttSeconds float64) {
	const zeroThreshold = 1.0e-6

	dist := c.coord.DistanceTo(other).Seconds()
	if rttSeconds < zeroThreshold {
		rttSeconds = zeroThreshold
	}
	wrongness := math.Abs(dist-rttSeconds) / rttSeconds

	totalError := c.coord.Error + other.Error
	if totalError < zeroThreshold {
		totalError = zeroThreshold
	}
	weight := c.coord.Error / totalError

	c.coord.Error = c.config.VivaldiCE*weight*wrongness + c.coord.Error*(1.0-c.config.VivaldiCE*weight)
	if c.coord.Error > c.config.VivaldiErrorMax {
		c.coord.Error = c.config.VivaldiErrorMax
	}

	delta := c.config.VivaldiCC * weight
	force := delta * (rttSeconds - dist)
	c.coord = c.coord.ApplyForce(force, other)
}

// updateAdjustment updates the adjustment portion of the client's coordinate, if
// the feature is enabled. This assumes that the mutex has been locked already.
func (c *Client) updateAdjustment(other *Coordinate, rttSeconds float64) {
	if c.config.AdjustmentWindowSize == 0 {
		return
	}

	// Note that the existing adjustment factors don't figure in to this
	// calculation so we use the raw distance here.
	dist := c.coord.rawDistanceTo(other)
	c.adjustmentSamples[c.adjustmentIndex] = rttSeconds - dist
	c.adjustmentIndex = (c.adjustmentIndex + 1) % c.config.AdjustmentWindowSize

	sum := 0.0
	for _, sample := range c.adjustmentSamples {
		sum += sample
	}
	c.coord.Adjustment = sum / (2.0*float64(c.config.AdjustmentWindowSize))
}

// Update takes other, a coordinate for another node, and rtt, a round trip
// time observation for a ping to that node, and updates the estimated position of
// the client's coordinate.
func (c *Client) Update(other *Coordinate, rtt time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	rttSeconds := rtt.Seconds()
	c.updateVivaldi(other, rttSeconds)
	c.updateAdjustment(other, rttSeconds)
}

// DistanceTo returns the estimated RTT from the client's coordinate to other, the
// coordinate for another node.
func (c *Client) DistanceTo(other *Coordinate) time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.coord.DistanceTo(other)
}
