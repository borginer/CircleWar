package protob

func BuildPlayerState(x, y float32) PlayerState {
	return PlayerState{
		Pos: &Position{X: x, Y: y},
	}
}

func BuildBulletState(x, y float32) BulletState {
	return BulletState{
		Pos: &Position{X: x, Y: y},
	}
}