package main

import (
	"crypto/sha256"
	"emmApi/models"
	"encoding/hex"
	"fmt"
	"github.com/RediSearch/redisearch-go/redisearch"
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"regexp"
	"time"
)

var AssetUrlRegex = regexp.MustCompile(`^(https?:\/\/)(api\.vrchat\.cloud|dbinj8iahsbec\.cloudfront\.net)\/(api\/1\/file|avatars)?\/(((file_)([0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12})\/\d\/file)?(.+?\.vrca)?)`)

var ErrResourceSharingConflict = fiber.Map{"error": "Resource sharing conflict."}
var ErrAvatarNotFound = fiber.Map{"error": "Avatar not found."}
var ErrInvalidAssetUrl = fiber.Map{"error": "Invalid asset URL."}
var ErrVRCPlusRequired = fiber.Map{"error": "VRChat, like emmVRC, relies on the support of their users to keep the platform free. Please support VRChat to unlock these features."}

func favoriteRoutes(router fiber.Router) {
	router.Get("/avatar", JwtRequired, EnforceModeration, GetAvatarFavorites)
	router.Post("/avatar", JwtRequired, EnforceModeration, AddAvatarFavorite)
	router.Delete("/avatar", JwtRequired, EnforceModeration, DeleteAvatarFavorite)
	router.Get("/avatar/export", JwtRequired, EnforceModeration, ExportFavorites)

	router.Put("/avatar", JwtRequired, EnforceModeration, PedestalScan)

	router.Get("/avatar/info/:hash", JwtRequired, EnforceModeration, GetAvatar)

	router.Get("/avatar/search", JwtRequired, EnforceModeration, SearchAvatars)
}

func GetAvatar(c *fiber.Ctx) error {
	var a models.Avatar

	avatarId := c.Params("hash")

	tx := DatabaseConnection.Where("avatar_id = ? OR avatar_id_sha256 = ?", avatarId, avatarId).First(&a)

	if tx.Error != nil {
		return c.Status(http.StatusNotFound).JSON(ErrAvatarNotFound)
	}

	return c.Status(http.StatusOK).JSON(a)
}

func SearchAvatars(c *fiber.Ctx) error {
	var l []models.LimitedAvatar

	docs, total, err := RediSearchClient.Search(redisearch.NewQuery(c.Query("q")).Limit(0, 50000))

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	if total == 0 {
		return c.JSON([0]models.LimitedAvatar{})
	}

	l = make([]models.LimitedAvatar, total)

	for i := 0; i < total; i++ {
		var a models.LimitedAvatar
		err := sonic.Unmarshal([]byte(docs[i].Properties["$"].(string)), &a)

		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}

		l[i] = a
	}

	return c.Status(http.StatusOK).JSON(l)
}

func ExportFavorites(c *fiber.Ctx) error {
	var favorites []models.AvatarFavorite
	var avatars []models.Avatar
	var export []AvatarExportResponse

	tx := DatabaseConnection.Preload(clause.Associations).Where("user_id = ?", c.Locals("userId").(string)).Order("id DESC").Find(&favorites)

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	avatars = make([]models.Avatar, len(favorites))

	for i := 0; i < len(favorites); i++ {
		avatars[i] = *favorites[i].Avatar
	}

	export = make([]AvatarExportResponse, len(avatars))

	for i := 0; i < len(avatars); i++ {
		export[i] = AvatarExportResponse{
			AvatarId:   avatars[i].AvatarId,
			AvatarName: avatars[i].AvatarName,
		}
	}

	return c.JSON(export)
}

func GetAvatarFavorites(c *fiber.Ctx) error {
	var favorites []models.AvatarFavorite
	//goland:noinspection GoPreferNilSlice
	var avatars = []models.Avatar{}

	tx := DatabaseConnection.Preload(clause.Associations).Where("user_id = ?", c.Locals("userId").(string)).Order("id DESC").Find(&favorites)

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	userId := c.Locals("userId").(string)

	for i := 0; i < len(favorites); i++ {
		avatar := *favorites[i].Avatar

		if userId != avatar.AvatarAuthorId && !avatar.AvatarPublic {
			continue
		}

		avatars = append(avatars, avatar)
	}

	return c.JSON(avatars)
}

func DeleteAvatarFavorite(c *fiber.Ctx) error {
	var f AvatarFavoriteRequest
	var a models.AvatarFavorite

	if err := c.BodyParser(&f); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	userId := c.Locals("userId").(string)

	tx := DatabaseConnection.Where("user_id = ? AND avatar_id = ?", userId, f.AvatarId).First(&a)

	if tx.Error == nil {
		tx = DatabaseConnection.Delete(&a)

		if tx.Error != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}
	} else {
		return c.Status(http.StatusNotFound).JSON(ErrAvatarNotFound)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func AddAvatarFavorite(c *fiber.Ctx) error {
	var f AvatarFavoriteRequest
	var a models.Avatar
	var u models.User

	if err := c.BodyParser(&f); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	userId := c.Locals("userId").(string)

	tx := DatabaseConnection.Where("user_id = ?", userId).First(&u)

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	if ServiceConfig.CheckService.CheckEnabled {
		isExpired := IsExpired(&u)

		if !isExpired && !u.HasVRCPlus {
			return c.Status(http.StatusPaymentRequired).JSON(ErrVRCPlusRequired)
		} else if isExpired {
			QueueUserCheck(userId)
		}
	}

	tx = DatabaseConnection.Where("avatar_id = ?", f.AvatarId).First(&a)

	if tx.Error == gorm.ErrRecordNotFound {
		if !AssetUrlRegex.MatchString(f.AvatarAssetUrl) {
			return c.Status(http.StatusBadRequest).JSON(ErrInvalidAssetUrl)
		}

		tx = DatabaseConnection.Where("avatar_thumbnail_url = ?", f.AvatarThumbnailUrl).First(&a)

		if tx.Error == nil {
			return c.Status(http.StatusBadRequest).JSON(ErrResourceSharingConflict)
		}

		avatarIdSha := sha256.Sum256([]byte(f.AvatarId))
		authorIdSha := sha256.Sum256([]byte(f.AvatarAuthorId))
		shaId := fmt.Sprintf("%s+%s", hex.EncodeToString(avatarIdSha[:]), hex.EncodeToString(authorIdSha[:]))
		a = models.Avatar{
			AvatarId:                 f.AvatarId,
			AvatarIdSha256:           shaId,
			AvatarName:               f.AvatarName,
			AvatarAuthorId:           f.AvatarAuthorId,
			AvatarAuthorName:         f.AvatarAuthorName,
			AvatarAssetUrl:           f.AvatarAssetUrl,
			AvatarThumbnailUrl:       f.AvatarThumbnailUrl,
			AvatarPublic:             f.AvatarPublic,
			AvatarSupportedPlatforms: f.AvatarSupportedPlatforms,
			AvatarSource:             models.Favorite,
			LastValidated:            time.Now(),
			IsDeleted:                false,
		}

		tx = DatabaseConnection.Create(&a)

		if tx.Error != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}

		err := IndexAvatar(&a)

		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}
	}

	var fa models.AvatarFavorite

	tx = DatabaseConnection.Where("user_id = ? AND avatar_id = ?", userId, f.AvatarId).First(&fa)

	if tx.Error == gorm.ErrRecordNotFound {
		fa = models.AvatarFavorite{
			UserId:   userId,
			AvatarId: a.AvatarId,
		}

		tx = DatabaseConnection.Create(&fa)

		if tx.Error != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}
	} else if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func PedestalScan(c *fiber.Ctx) error {
	var f AvatarFavoriteRequest
	var a models.Avatar

	if err := c.BodyParser(&f); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	tx := DatabaseConnection.Where("avatar_id = ?", f.AvatarId).First(&a)

	if tx.Error == gorm.ErrRecordNotFound {
		if !AssetUrlRegex.MatchString(f.AvatarAssetUrl) {
			return c.Status(http.StatusBadRequest).JSON(ErrInvalidAssetUrl)
		}

		tx = DatabaseConnection.Where("avatar_thumbnail_url = ?", f.AvatarThumbnailUrl).First(&a)

		if tx.Error == nil {
			return c.Status(http.StatusBadRequest).JSON(ErrResourceSharingConflict)
		}

		avatarIdSha := sha256.Sum256([]byte(f.AvatarId))
		authorIdSha := sha256.Sum256([]byte(f.AvatarAuthorId))
		shaId := fmt.Sprintf("%s+%s", hex.EncodeToString(avatarIdSha[:]), hex.EncodeToString(authorIdSha[:]))
		a = models.Avatar{
			AvatarId:                 f.AvatarId,
			AvatarIdSha256:           shaId,
			AvatarName:               f.AvatarName,
			AvatarAuthorId:           f.AvatarAuthorId,
			AvatarAuthorName:         f.AvatarAuthorName,
			AvatarAssetUrl:           f.AvatarAssetUrl,
			AvatarThumbnailUrl:       f.AvatarThumbnailUrl,
			AvatarPublic:             f.AvatarPublic,
			AvatarSupportedPlatforms: f.AvatarSupportedPlatforms,
			AvatarSource:             models.Pedestal,
			LastValidated:            time.Now(),
			IsDeleted:                false,
		}

		tx = DatabaseConnection.Create(&a)

		if tx.Error != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}

		err := IndexAvatar(&a)

		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func IndexAvatar(a *models.Avatar) error {
	var b models.BlacklistedAuthor

	tx := DatabaseConnection.Where("user_id = ?", a.AvatarAuthorId).First(&b)

	if tx.Error == gorm.ErrRecordNotFound {
		res, err := ReJsonClient.JSONSet(a.AvatarIdSha256, "$", a.GetLimitedAvatar())

		if err != nil {
			return err
		}

		if res.(string) != "OK" {
			fmt.Printf("Error adding avatar to search index: %s\n", a.AvatarIdSha256)
		}
	}

	return nil
}
