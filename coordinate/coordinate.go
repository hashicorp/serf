package coordinate

import (
	"log"
	"math"
	"math/rand"
)

const (
	HEIGHT_THRESHOLD = 0.01
)

type Coordinate struct {
	vec    []float64
	height float64
}

func NewCoordinate(dimension uint) Coordinate {
	return Coordinate{
		vec:    make([]float64, dimension),
		height: HEIGHT_THRESHOLD,
	}
}

func (self Coordinate) Add(other Coordinate) Coordinate {
	if len(self.vec) != len(other.vec) {
		log.Fatalf("adding two coordinates that have different dimensions:\n%+v\n%+v", self, other)
		return Coordinate{}
	} else {
		ret := NewCoordinate(uint(len(self.vec)))

		if ret.height < HEIGHT_THRESHOLD {
			ret.height = HEIGHT_THRESHOLD
		}

		for i, _ := range self.vec {
			ret.vec[i] = self.vec[i] + other.vec[i]
		}

		return ret
	}
}

func (self Coordinate) Sub(other Coordinate) Coordinate {
	if len(self.vec) != len(other.vec) {
		log.Fatalf("subtracting two coordinates that have different dimensions:\n%+v\n%+v", self, other)
		return Coordinate{}
	} else {
		ret := NewCoordinate(uint(len(self.vec)))

		ret.height = self.height + other.height

		for i, _ := range self.vec {
			ret.vec[i] = self.vec[i] - other.vec[i]
		}

		return ret
	}
}

func (self Coordinate) Mul(factor float64) Coordinate {
	ret := NewCoordinate(uint(len(self.vec)))

	ret.height = self.height * float64(factor)
	if ret.height < HEIGHT_THRESHOLD {
		ret.height = HEIGHT_THRESHOLD
	}

	for i, _ := range self.vec {
		ret.vec[i] = self.vec[i] * float64(factor)
	}

	return ret
}

func (self Coordinate) DistanceTo(other Coordinate) float64 {
	tmp := self.Sub(other)
	sum := 0.0
	for i, _ := range self.vec {
		sum += math.Pow(tmp.vec[i], 2)
	}
	return math.Sqrt(sum) + tmp.height
}

func (self Coordinate) DirectionTo(other Coordinate) Coordinate {
	tmp := self.Sub(other)
	dist := self.DistanceTo(other)
	if dist != self.height+other.height {
		tmp = tmp.Mul(1.0 / dist)
		return tmp
	} else {
		for i, _ := range self.vec {
			tmp.vec[i] = (10-0.1)*rand.Float64() + 0.1
		}
		dist = tmp.DistanceTo(NewCoordinate(uint(len(self.vec))))
		tmp = tmp.Mul(1.0 / dist)
		return tmp
	}
}
