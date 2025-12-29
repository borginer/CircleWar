package hitboxes

import (
	"CircleWar/config"
	sharedtypes "CircleWar/shared/types"
)

func PlayerSize(health sharedtypes.PlayerHealth) float32 {
	return config.InitialPlayerSize + (float32(health)-config.InitialPlayerHealth)*config.PlayerShrinkStep
}

func BulletSize(health sharedtypes.PlayerHealth) float32 {
	return config.InitialBulletSize + (float32(health)-config.InitialPlayerHealth)*config.BulletShrinkStep
}
