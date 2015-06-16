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

	// mutex enables safe concurrent access to the client.
	mutex *sync.RWMutex
}

const (
	// secondsToNanoseconds is used to convert float seconds to nanoseconds.
	secondsToNanoseconds = 1.0e9
)

// NewClient creates a new Client and verifies the configuration is valid.
func NewClient(config *Config) (*Client, error) {
	if !(config.Dimensionality > 0) {
		return nil, fmt.Errorf("dimensionality must be >0")
	}

	return &Client{
		coord:  NewCoordinate(config),
		config: config,
		mutex:  &sync.RWMutex{},
	}, nil
}

// GetCoordinate returns a copy of the coordinate for this client.
func (c *Client) GetCoordinate() *Coordinate {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.coord.Clone()
}

// Update takes other, a coordinate for another node, and rtt, a round trip
// time observation for a ping to that node, and updates the estimated position of
// the client's coordinate.
func (c *Client) Update(other *Coordinate, rtt time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	const zeroThreshold = 1.0e-6

	dist := c.coord.DistanceTo(other)
	rttSeconds := rtt.Seconds()
	if rttSeconds < zeroThreshold {
		rttSeconds = zeroThreshold
	}
	wrongness := math.Abs(dist-rttSeconds) / rttSeconds

	totalError := c.coord.Error + other.Error
	if totalError < zeroThreshold {
		totalError = zeroThreshold
	}
	weight := c.coord.Error / totalError

	c.coord.Error = c.config.VivaldiCE*weight*wrongness + c.coord.Error*(1-c.config.VivaldiCE*weight)
	if c.coord.Error > c.config.VivaldiErrorMax {
		c.coord.Error = c.config.VivaldiErrorMax
	}

	delta := c.config.VivaldiCC * weight
	force := delta * (rttSeconds - dist)
	c.coord = c.coord.ApplyForce(force, other)
}

// DistanceTo returns the estimated RTT from the client's coordinate to other, the
// coordinate for another node.
func (c *Client) DistanceTo(other *Coordinate) time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	dist := c.coord.DistanceTo(other) * secondsToNanoseconds
	return time.Duration(dist)
}
