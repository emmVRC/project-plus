package models

import "time"

type Ban struct {
	BanId      string    `gorm:"primaryKey" json:"ban_id"`
	BanUserId  string    `json:"-" gorm:"index"`
	BanIssuer  string    `json:"-" gorm:"index"`
	BanReason  string    `json:"ban_reason"`
	IpAddress  string    `json:"-"`
	BanExpires time.Time `json:"ban_expires"`
	BanUpdated time.Time `json:"-"`
	BanCreated time.Time `json:"-"`
}
