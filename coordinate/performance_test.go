package coordinate

import (
	"testing"
	"time"
)

func TestPerformance_Line(t *testing.T) {
	const spacing = 10 * time.Millisecond
	const nodes, cycles = 10, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateLine(nodes, spacing)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.0016 || stats.ErrorMax > 0.0068 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestPerformance_Grid(t *testing.T) {
	const spacing = 10 * time.Millisecond
	const nodes, cycles = 25, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateGrid(nodes, spacing)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.0015 || stats.ErrorMax > 0.022 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestPerformance_Split(t *testing.T) {
	const lan, wan = 1 * time.Millisecond, 10 * time.Millisecond
	const nodes, cycles = 25, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateSplit(nodes, lan, wan)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.000062 || stats.ErrorMax > 0.00045 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestPerformance_Height(t *testing.T) {
	const radius = 100 * time.Millisecond
	const nodes, cycles = 25, 1000

	// Constrain us to two dimensions so that we can just exactly represent
	// the circle.
	config := DefaultConfig()
	config.Dimensionality = 2
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}

	// Generate truth where the first coordinate is in the "middle" because
	// it's equidistant from all the nodes, but it will have an extra radius
	// added to the distance, so it should come out above all the others.
	truth := GenerateCircle(nodes, radius)
	Simulate(clients, truth, cycles, nil)

	// Make sure the height looks reasonable with the regular nodes all in a
	// plane, and the center node up above.
	for i, _ := range clients {
		coord := clients[i].GetCoordinate()
		if i == 0 {
			if coord.Height < 0.97*radius.Seconds() {
				t.Fatalf("height is out of spec: %9.6f", coord.Height)
			}
		} else {
			if coord.Height > 0.03*radius.Seconds() {
				t.Fatalf("height is out of spec: %9.6f", coord.Height)
			}
		}
	}
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.0025 || stats.ErrorMax > 0.064 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestPerformance_Random(t *testing.T) {
	const mean, deviation = 100 * time.Millisecond, 10 * time.Millisecond
	const nodes, cycles = 25, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateRandom(nodes, mean, deviation)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.075 || stats.ErrorMax > 0.33 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}
