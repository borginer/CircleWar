package gamepb

func BuildPlayerState(x, y float32) PlayerState {
	return PlayerState{
		Pos: &Position{X: x, Y: y},
	}
}
