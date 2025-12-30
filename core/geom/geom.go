package geom

import "math"

type Vector2 struct{ X, Y float32 }

func NewVector(x, y float32) Vector2 {
	return Vector2{x, y}
}

func (v Vector2) Add(x, y float32) Vector2 {
	return Vector2{v.X + x, v.Y + y}
}

func (v Vector2) Sub(other Vector2) Vector2 {
	return NewVector(v.X-other.X, v.Y-other.Y)
}

func (v Vector2) DistTo(other Vector2) float32 {
	return float32(math.Sqrt(math.Pow(float64(v.X-other.X), 2) + math.Pow(float64(v.Y-other.Y), 2)))
}

func limit(value, ll, ul float32) float32 {
	if value < ll {
		return ll
	} else if value > ul {
		return ul
	}
	return value
}

// limits vector coordinates to the given parameters (a square)
func (v Vector2) Limited(lx, ly, ux, uy float32) Vector2 {
	return NewVector(limit(v.X, lx, ux), limit(v.Y, ly, uy))
}

func (v Vector2) normalized() Vector2 {
	length := math.Sqrt(math.Pow(float64(v.X), 2) + math.Pow(float64(v.Y), 2))
	if length == 0 {
		return v // (0, 0)
	}
	return Vector2{v.X / float32(length), v.Y / float32(length)}
}

func (v Vector2) Coord() (float32, float32) {
	return v.X, v.Y
}

type Direction Vector2

func NewDir(v Vector2) Direction {
	return Direction(v.normalized())
}

func (d Direction) ScalarMult(by float32) Vector2 {
	return NewVector(d.X*by, d.Y*by)
}
