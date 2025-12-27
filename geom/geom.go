package geom

import "math"

type Position vector2
type Direction vector2
type vector2 struct{ X, Y float32 }

func NewPosition(x, y float32) Position {
	return Position{x, y}
}

func (p Position) Add(x, y float32) Position {
	return Position{p.X + x, p.Y + y}
}

func NewDir(x, y float32) Direction {
	return Direction(vector2{x, y}.normalized())
}

func (v vector2) normalized() vector2 {
	length := math.Sqrt(math.Pow(float64(v.X), 2) + math.Pow(float64(v.Y), 2))
	return vector2{v.X / float32(length), v.Y / float32(length)}
}
