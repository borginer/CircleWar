package geom

import "math"

type Position Vector2
type Direction Vector2
type Vector2 struct{ X, Y float32 }

func NewPosition(x, y float32) Position {
	return Position{x, y}
}

func (p Position) Add(x, y float32) Position {
	return Position{p.X + x, p.Y + y}
}

func (p Position) DistTo(other Position) float32 {
	return float32(math.Sqrt(math.Pow(float64(p.X-other.X), 2) + math.Pow(float64(p.Y-other.Y), 2)))
}

func NewDir(x, y float32) Direction {
	return Direction(Vector2{x, y}.normalized())
}

func (v Vector2) normalized() Vector2 {
	length := math.Sqrt(math.Pow(float64(v.X), 2) + math.Pow(float64(v.Y), 2))
	if length == 0 {
		return Vector2{0, 0}
	}
	return Vector2{v.X / float32(length), v.Y / float32(length)}
}

func (v Vector2) Add(x, y float32) Vector2 {
	return Vector2{v.X + x, v.Y + y}
}

func NewVector(x, y float32) Vector2 {
	return Vector2{x, y}
}
