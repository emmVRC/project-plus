package models

import (
	"time"
)

type Avatar struct {
	AvatarId                 string       `gorm:"primaryKey" json:"avatar_id"`
	AvatarIdSha256           string       `json:"-" gorm:"index"`
	AvatarName               string       `json:"avatar_name" gorm:"index"`
	AvatarAuthorId           string       `json:"avatar_author_id"`
	AvatarAuthorName         string       `json:"avatar_author_name" gorm:"index"`
	AvatarAssetUrl           string       `json:"avatar_asset_url" gorm:"index"`
	AvatarThumbnailUrl       string       `json:"avatar_thumbnail_url" gorm:"index"`
	AvatarPublic             bool         `json:"avatar_public"`
	AvatarSupportedPlatforms int          `json:"avatar_supported_platforms"`
	AvatarSource             AvatarSource `json:"-"`
	LastValidated            time.Time    `json:"-"`
	IsDeleted                bool         `json:"-"`
}

type AvatarSource int32

const (
	Import   AvatarSource = 0
	Favorite              = 1
	Pedestal              = 2
)

type LimitedAvatar struct {
	AvatarId                 string `json:"avatar_id"`
	AvatarName               string `json:"avatar_name"`
	AvatarAuthorId           string `json:"avatar_author_id"`
	AvatarAuthorName         string `json:"avatar_author_name"`
	AvatarThumbnailUrl       string `json:"avatar_thumbnail_url"`
	AvatarPublic             bool   `json:"avatar_public"`
	AvatarSupportedPlatforms int    `json:"avatar_supported_platforms"`
}

func (a *Avatar) GetLimitedAvatar() *LimitedAvatar {
	return &LimitedAvatar{
		AvatarId:                 a.AvatarIdSha256,
		AvatarName:               a.AvatarName,
		AvatarAuthorId:           a.AvatarAuthorId,
		AvatarAuthorName:         a.AvatarAuthorName,
		AvatarThumbnailUrl:       a.AvatarThumbnailUrl,
		AvatarPublic:             a.AvatarPublic,
		AvatarSupportedPlatforms: a.AvatarSupportedPlatforms,
	}
}
