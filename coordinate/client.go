package coordinate

import (
	"math"
	"sync"
	"time"
)

// Config is used to set the parameters of the Vivaldi-based coordinate mapping
// algorithm. The following references are called out at various points in the
// documentation here:
//
// [1] Dabek, Frank, et al. "Vivaldi: A decentralized network coordinate system."
//     ACM SIGCOMM Computer Communication Review. Vol. 34. No. 4. ACM, 2004.
// [2] Ledlie, Jonathan, Paul Gardner, and Margo I. Seltzer. "Network Coordinates
//     in the Wild." NSDI. Vol. 7. 2007.
// [3] Lee, Sanghwan, et al. "On suitability of Euclidean embedding for
//     host-based network coordinate systems." Networking, IEEE/ACM Transactions
//     on 18.1 (2010): 27-40.
type Config struct {
	// The dimensionality of the coordinate system. As discussed in [2], more
	// dimensions improves the accuracy of the estimates up to a point. In
	// particular, there was no noticeable improvement beyond 7 dimensions.
	Dimensionality uint

	// MaxVivaldiError is the default error value when a node hasn't yet made
	// any observations. It also serves as an upper limit on the error value in
	// case observations cause the error value to increase.
	MaxVivaldiError    float64

	// VivaldiCE is a tuning factor that controls the maximum impact an
	// observation can have on a node's confidence. See [1] for more details.
	VivaldiCE          float64

	// VivaldiCC is a tuning factor that controls the maximum impact an
	// observation can have on a node's coordinate. See [1] for more details.
	VivaldiCC          float64

	// AdjustmentWindowSize sets the number of observations we use to update
	// the adjustment term per the technique described in [3]. Setting this
	// to 0 will disable the adjustment feature. In the future we may want to
	// dynamically adjust this parameter based on the cluster size and gossip
	// rate.
	AdjustmentWindowSize uint
}

// DefaultConfig returns a Config that has some default values suitable for
// basic testing of the algorithm, but not tuned to any particular type of cluster.
func DefaultConfig() *Config {
	return &Config{
		Dimensionality:       8,
		MaxVivaldiError:      1.5,
		VivaldiCE:            0.25,
		VivaldiCC:            0.25,
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
func NewClient(config *Config) (*Client, error) {
	// TODO - check the config here.

	return &Client{
		coord:             NewCoordinate(config),
		config:            config,
		adjustment_index:  0,
		adjustment_window: make([]float64, config.AdjustmentWindowSize),
		mutex:             &sync.RWMutex{},
	}, nil
}

// GetCoordinate returns a copy of the coordinate of this client
func (c *Client) GetCoordinate() *Coordinate {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.coord.Clone()
}

// Update takes a Client, which contains the position of another node, and the rtt between the receiver
// and the other node, and updates the position of the receiver.
func (c *Client) Update(coord *Coordinate, rttDur time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	rtt := float64(rttDur.Nanoseconds()) / (1000 * 1000) // 1 millisecond = 1000 * 1000 nanoseconds
	dist := c.coord.DistanceTo(coord)

	weight := c.coord.Err / (c.coord.Err + coord.Err)
	err_calc := math.Abs(dist-rtt) / rtt
	c.coord.Err = err_calc*c.config.VivaldiCE*weight + c.coord.Err*(1-c.config.VivaldiCE*weight)
	if c.coord.Err > c.config.MaxVivaldiError {
		c.coord.Err = c.config.MaxVivaldiError
	}
	delta := c.config.VivaldiCC * weight

	force := delta*(rtt-dist)
	c.coord = c.coord.ApplyForce(force, coord)
	c.updateAdjustment(coord, rtt)
	return nil
}

func (c *Client) updateAdjustment(coord *Coordinate, rtt float64) error {
	if c.config.AdjustmentWindowSize > 0 {
		dist := c.coord.DistanceTo(coord)
		c.adjustment_window[c.adjustment_index] = rtt - dist
		c.adjustment_index = (c.adjustment_index + 1) % c.config.AdjustmentWindowSize
		tmp := 0.0
		for _, n := range c.adjustment_window {
			tmp += n
		}
		c.coord.Adjustment = tmp / (2.0 * float64(c.config.AdjustmentWindowSize))
	}
	return nil
}

// DistanceTo takes a Client, which contains the position of another node, and computes the distance
// between the receiver and the other node.
func (c *Client) DistanceTo(coord *Coordinate) (time.Duration) {
	my_coord := c.GetCoordinate()
	dist := my_coord.DistanceTo(coord)
	return time.Duration(dist+my_coord.Adjustment+coord.Adjustment) * time.Millisecond
}
