package coordinate

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// GenerateClients returns a slice with nodes number of clients, all with the
// given config.
func GenerateClients(nodes int, config *Config) ([]*Client, error) {
	clients := make([]*Client, nodes)
	for i, _ := range clients {
		client, err := NewClient(config)
		if err != nil {
			return nil, err
		}

		clients[i] = client
	}
	return clients, nil
}

// GenerateLine returns a truth matrix as if all the nodes are in a straight linke
// with the given spacing between them.
func GenerateLine(nodes int, spacing time.Duration) [][]time.Duration {
	truth := make([][]time.Duration, nodes)
	for i := range truth {
		truth[i] = make([]time.Duration, nodes)
	}

	for i := 0; i < nodes; i++ {
		for j := i + 1; j < nodes; j++ {
			rtt := time.Duration(j-i) * spacing
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// GenerateGrid returns a truth matrix as if all the nodes are in a two dimensional
// grid with the given spacing between them.
func GenerateGrid(nodes int, spacing time.Duration) [][]time.Duration {
	truth := make([][]time.Duration, nodes)
	for i := range truth {
		truth[i] = make([]time.Duration, nodes)
	}

	n := int(math.Sqrt(float64(nodes)))
	for i := 0; i < nodes; i++ {
		for j := i + 1; j < nodes; j++ {
			x1, y1 := float64(i%n), float64(i/n)
			x2, y2 := float64(j%n), float64(j/n)
			dx, dy := x2-x1, y2-y1
			dist := math.Sqrt(dx*dx + dy*dy)
			rtt := time.Duration(dist * float64(spacing))
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// GenerateSplit returns a truth matrix as if half the nodes are close together in
// one location and half the nodes are close together in another. The lan factor
// is used to separate the nodes locally and the wan factor represents the split
// between the two sides.
func GenerateSplit(nodes int, lan time.Duration, wan time.Duration) [][]time.Duration {
	truth := make([][]time.Duration, nodes)
	for i := range truth {
		truth[i] = make([]time.Duration, nodes)
	}

	split := nodes / 2
	for i := 0; i < nodes; i++ {
		for j := i + 1; j < nodes; j++ {
			rtt := lan
			if (i <= split && j > split) || (i > split && j <= split) {
				rtt += wan
			}
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// GenerateRandom returns a truth matrix for a set of nodes with random delays, up
// to the given max. The RNG is re-seeded so you always get the same matrix for a
// given size.
func GenerateRandom(nodes int, max time.Duration) [][]time.Duration {
	rand.Seed(1)

	truth := make([][]time.Duration, nodes)
	for i := range truth {
		truth[i] = make([]time.Duration, nodes)
	}

	for i := 0; i < nodes; i++ {
		for j := i + 1; j < nodes; j++ {
			rtt := time.Duration(rand.Float64() * float64(max))
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// SimCycleFn will get called for each cycle of Simulate to allow users to evaluate
// the progress of the algorithm over time.
type SimCycleFn func(cycle int, clients []*Client, truth [][]time.Duration)

// Simulate runs the given number of cycles using the given list of clients and
// truth matrix. On each cycle, each client will pick a random node and observe
// the truth RTT, updating its coordinate estimate. An optional callback will be
// called each cycle to evaluate process (this can be nil). The RNG is re-seeded
// for each simulation run to get deterministic results (for this algorithm and
// the underlying algorithm which will use random numbers for position vectors
// when starting out with everything at the origin).
func Simulate(clients []*Client, truth [][]time.Duration, cycles int, callback SimCycleFn) {
	rand.Seed(1)

	nodes := len(clients)
	for cycle := 0; cycle < cycles; cycle++ {
		if callback != nil {
			callback(cycle, clients, truth)
		}

		for i, _ := range clients {
			if j := rand.Intn(nodes); j != i {
				c := clients[j].GetCoordinate()
				rtt := truth[i][j]
				clients[i].Update(c, rtt)
			}
		}
	}
}

// Stats is returned from the Evaluate function with a summary of the algorithm
// performance.
type Stats struct {
	ErrorMax float64
	ErrorAvg float64
}

// Evaluate uses the coordinates of the given clients to calculate estimated
// distances and compares them with the given truth matrix, returning summary
// stats.
func Evaluate(clients []*Client, truth [][]time.Duration) (stats Stats) {
	nodes := len(clients)
	count := 0
	for i := 0; i < nodes; i++ {
		for j := i + 1; j < nodes; j++ {
			est := clients[i].DistanceTo(clients[j].GetCoordinate()).Seconds()
			actual := truth[i][j].Seconds()
			error := math.Abs(est-actual) / actual
			stats.ErrorMax = math.Max(stats.ErrorMax, error)
			stats.ErrorAvg += error
			count += 1
		}
	}

	stats.ErrorAvg /= float64(count)
	fmt.Printf("Error avg=%9.6f max=%9.6f\n", stats.ErrorAvg, stats.ErrorMax)
	return
}
