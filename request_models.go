package main

import "github.com/gofiber/fiber/v2"

var ErrInvalidRequestBody = fiber.Map{"error": "Invalid request body."}
var ErrInternalServerError = fiber.Map{"error": "Internal server error."}

type AuthenticationRequest struct {
	UserId              string `json:"user_id"`
	DisplayName         string `json:"display_name"`
	Password            string `json:"password"`
	PersistentToken     string `json:"persistent_token"`
	NeedPersistentToken bool   `json:"need_persistent_token"`
}

type AuthenticationResponse struct {
	Token           string `json:"token"`
	PersistentToken string `json:"persistent_token"`
}

type PasswordResetRequest struct {
	UserId      string `json:"user_id"`
	Password    string `json:"password"`
	NewPassword string `json:"new_password"`
}

type AvatarFavoriteRequest struct {
	AvatarId                 string `json:"avatar_id"`
	AvatarName               string `json:"avatar_name"`
	AvatarAuthorId           string `json:"avatar_author_id"`
	AvatarAuthorName         string `json:"avatar_author_name"`
	AvatarAssetUrl           string `json:"avatar_asset_url"`
	AvatarThumbnailUrl       string `json:"avatar_thumbnail_url"`
	AvatarPublic             bool   `json:"avatar_public"`
	AvatarSupportedPlatforms int    `json:"avatar_supported_platforms"`
}

type AvatarExportResponse struct {
	AvatarId   string `json:"avatar_id"`
	AvatarName string `json:"avatar_name"`
}
