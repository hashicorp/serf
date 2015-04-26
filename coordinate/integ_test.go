package coordinate

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

const (
	ONE_HUNDRED_MILLISECONDS_IN_NANOSECONDS  = 100 * 1000 * 1000
	ONE_THOUSAND_MILLISECONDS_IN_NANOSECONDS = 10 * ONE_HUNDRED_MILLISECONDS_IN_NANOSECONDS

	ERR_STD = 0.2
)

func generateLatencyMatrix(numNodes int) [][]time.Duration {
	matrix := make([][]time.Duration, numNodes)
	for i := range matrix {
		matrix[i] = make([]time.Duration, numNodes)
	}

	for i := range matrix {
		for j := i; j < numNodes; j++ {
			if i != j {
				matrix[i][j] = time.Duration(rand.NormFloat64()*ONE_HUNDRED_MILLISECONDS_IN_NANOSECONDS + ONE_THOUSAND_MILLISECONDS_IN_NANOSECONDS)
				matrix[j][i] = matrix[i][j]
			}
		}
	}

	return matrix
}

// Return a random duration that is between 0.8 to 1.2 times the given duration
func perturb(n time.Duration) time.Duration {
	return time.Duration(float64(n.Nanoseconds()) * (rand.NormFloat64()*ERR_STD + 1))
}

func TestAlgorithm(t *testing.T) {
	numNodes := 100
	matrix := generateLatencyMatrix(numNodes)

	nodes := make([]*Client, numNodes)
	for i := 0; i < numNodes; i++ {
		nodes[i] = NewClient(DefaultConfig())
	}

	for i := 0; i < 10000; i++ {
		for j := range nodes {
			m := rand.Intn(numNodes)
			if j != m {
				nodes[j].Update(nodes[m].GetCoordinate(), perturb(matrix[j][m]))
			}
		}
	}

	counter := 0.0
	totalErr := 0.0
	for i := range nodes {
		for j := range nodes {
			if i != j {
				dist, err := nodes[i].DistanceTo(nodes[j].GetCoordinate())
				if err != nil {
					t.Fatal(err)
				}
				totalErr += math.Abs((dist - matrix[i][j]).Seconds()) / math.Abs(matrix[i][j].Seconds())
				counter += 1
			}
		}
	}

	averageErr := totalErr / counter
	if averageErr > ERR_STD {
		t.Fatalf("average error is too large: %f", averageErr)
	}
}
