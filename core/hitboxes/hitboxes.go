package hitboxes

import (
	"CircleWar/config"
	"CircleWar/core/stypes"
)

func PlayerSize(health stypes.PlayerHealth) float32 {
	return config.InitialPlayerSize + (float32(health)-config.InitialPlayerHealth)*config.PlayerShrinkStep
}

func BulletSize(health stypes.PlayerHealth) float32 {
	return config.InitialBulletSize + (float32(health)-config.InitialPlayerHealth)*config.BulletShrinkStep
}
