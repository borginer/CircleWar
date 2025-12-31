package config

const ServerIP = "127.0.0.1"
const Port = 4000

const WorldWidth = 1020
const WorldHeight = 680

const BulletSpeed = 1800
const PlayerSpeed = 1100

const BulletTimeToLiveSec = 1.5
const BulletCooldownMS = 180

// players and bullets shrink as the player losses hp
const InitialPlayerHealth = 20
const InitialPlayerSize = 48
const PlayerShrinkStep = 1

const InitialBulletSize = 20
const BulletShrinkStep = 0.5
