package models

import (
	"github.com/lib/pq"
	"time"
)

type User struct {
	UserId           string `gorm:"primaryKey"`
	UserPin          string
	UserKnownAliases pq.StringArray `gorm:"type:text[] NOT NULL;default: '{}'::text[]"`
	HasVRCPlus       bool
	LastVRCPlusCheck time.Time
	LastSeen         time.Time
	CreatedAt        time.Time
}

type PersistentToken struct {
	ID     uint   `gorm:"primaryKey"`
	UserId string `gorm:"index" gorm:"foreignKey:UserId"`
	Token  string `gorm:"index"`
}
