package hitboxes

import (
	"CircleWar/config"
	"CircleWar/core/netmsg"
)

func PlayerSize(health netmsg.PlayerHealth) float32 {
	return config.InitialPlayerSize + (float32(health)-config.InitialPlayerHealth)*config.PlayerShrinkStep
}

func BulletSize(health netmsg.PlayerHealth) float32 {
	return config.InitialBulletSize + (float32(health)-config.InitialPlayerHealth)*config.BulletShrinkStep
}
