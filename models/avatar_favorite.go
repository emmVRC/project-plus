package models

import (
	"time"
)

type AvatarFavorite struct {
	ID        uint    `gorm:"primaryKey"`
	UserId    string  `gorm:"index" gorm:"foreignKey:UserId"`
	User      *User   `gorm:"references:UserId"`
	AvatarId  string  `gorm:"foreignKey:AvatarId"`
	Avatar    *Avatar `gorm:"references:AvatarId"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BlacklistedAuthor struct {
	UserId string `gorm:"primaryKey"`
}
