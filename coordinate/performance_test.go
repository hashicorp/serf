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
	if stats.ErrorAvg > 0.0012 || stats.ErrorMax > 0.0055 {
		t.Fatalf("performance stats are out of spec: %v", stats)
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
	if stats.ErrorAvg > 0.0014 || stats.ErrorMax > 0.0158 {
		t.Fatalf("performance stats are out of spec: %v", stats)
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
	if stats.ErrorAvg > 0.000057 || stats.ErrorMax > 0.00035 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestPerformance_Random(t *testing.T) {
	const mean, deviation = 100*time.Millisecond, 10*time.Millisecond
	const nodes, cycles = 25, 1000
	config := DefaultConfig()
	clients, err := GenerateClients(nodes, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateRandom(nodes, mean, deviation)
	Simulate(clients, truth, cycles, nil)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.046 || stats.ErrorMax > 0.184 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}
