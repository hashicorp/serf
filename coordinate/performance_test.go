package coordinate

import (
	"testing"
	"time"
)

func TestPerformance_Line(t *testing.T) {
	const spacing = 10*time.Millisecond
	const nodes, cycles = 10, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateLine(nodes, spacing)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.005 {
		t.Fatalf("average error is too large, %9.6f", stats.ErrorAvg)
	}
}

func TestPerformance_Grid(t *testing.T) {
	const spacing = 10*time.Millisecond
	const nodes, cycles = 25, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateGrid(nodes, spacing)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.006 {
		t.Fatalf("average error is too large, %9.6f", stats.ErrorAvg)
	}
}

func TestPerformance_Split(t *testing.T) {
	const lan, wan = 1*time.Millisecond, 10*time.Millisecond
	const nodes, cycles = 25, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateSplit(nodes, lan, wan)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.045 {
		t.Fatalf("average error is too large, %9.6f", stats.ErrorAvg)
	}
}

func TestPerformance_Random(t *testing.T) {
	const max = 10*time.Millisecond
	const nodes, cycles = 25, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateRandom(nodes, max)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)

	// TODO - Currently horrible! Height and the adjustment factor should
	// help here, so revisit once those are in.
	if stats.ErrorAvg > 4.8 {
		t.Fatalf("average error is too large, %9.6f", stats.ErrorAvg)
	}
}
