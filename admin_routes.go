package main

import (
	"context"
	"emmApi/models"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strconv"
	"strings"
)

var ctx = context.Background()

var ErrAlreadyRebuilding = fiber.Map{"error": "Search index rebuild is already running. Please wait."}

var ErrUserNotFound = fiber.Map{"error": "User not found."}
var ErrUserAlreadyBlacklisted = fiber.Map{"error": "User is already blacklisted."}
var ErrUserIsNotBlacklisted = fiber.Map{"error": "User is not blacklisted."}

func adminRoutes(router fiber.Router) {
	router.Post("/admin/rebuild_search_index", EnforceAdminSecret, RebuildSearchIndex)
	router.Get("/admin/rebuild_search_index/status", EnforceAdminSecret, RebuildSearchIndexStatus)

	router.Post("/admin/reset_user_pin", EnforceAdminSecret, ResetUserPin)
	router.Post("/admin/export_user_favorites", EnforceAdminSecret, ExportFavoritesAdmin)
	router.Delete("/admin/delete_user", EnforceAdminSecret, DeleteUser)

	router.Get("/admin/avatar/:avatar_id", EnforceAdminSecret, GetAdminAvatar)

	router.Post("/admin/blacklist_avatar/:avatar_id", EnforceAdminSecret, BlacklistAvatar)
	router.Post("/admin/blacklist_author", EnforceAdminSecret, BlacklistAvatarAuthor)
	router.Delete("/admin/blacklist_author", EnforceAdminSecret, UnBlacklistAvatarAuthor)
}

func EnforceAdminSecret(c *fiber.Ctx) error {
	authorizationHeader := c.Get("Authorization")

	if !strings.HasPrefix(authorizationHeader, "Bearer ") {
		return c.Status(http.StatusUnauthorized).
			JSON(ErrMissingBearerToken)
	}

	authorizationHeader = strings.TrimPrefix(authorizationHeader, "Bearer ")

	if authorizationHeader != ServiceConfig.AdminSecret {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{})
	}

	return c.Next()
}

func DeleteUser(c *fiber.Ctx) error {
	var r GenericUserRequest
	var u models.User

	if err := c.BodyParser(&r); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	tx := DatabaseConnection.Where("user_id = ?", r.UserId).First(&u)

	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	} else if tx.Error == gorm.ErrRecordNotFound {
		return c.Status(http.StatusBadRequest).JSON(ErrUserNotFound)
	}

	tx = DatabaseConnection.Where("user_id = ?", r.UserId).Delete(&models.AvatarFavorite{})

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	tx = DatabaseConnection.Where("user_id = ?", r.UserId).Delete(&models.PersistentToken{})

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	tx = DatabaseConnection.Delete(&u)

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	return c.Status(http.StatusNoContent).JSON(fiber.Map{})
}

func ExportFavoritesAdmin(c *fiber.Ctx) error {
	var r GenericUserRequest
	var favorites []models.AvatarFavorite
	var avatars []models.Avatar
	var export []AvatarExportResponse

	if err := c.BodyParser(&r); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	tx := DatabaseConnection.Preload(clause.Associations).Where("user_id = ?", r.UserId).Order("id DESC").Find(&favorites)

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

func ResetUserPin(c *fiber.Ctx) error {
	var r GenericUserRequest
	var u models.User

	if err := c.BodyParser(&r); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	tx := DatabaseConnection.Where("user_id = ?", r.UserId).First(&u)

	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	} else if tx.Error == gorm.ErrRecordNotFound {
		return c.Status(http.StatusNotFound).JSON(ErrUserNotFound)
	}

	u.UserPin = ""

	tx = DatabaseConnection.Save(&u)

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func UnBlacklistAvatarAuthor(c *fiber.Ctx) error {
	var r GenericUserRequest
	var b models.BlacklistedAuthor

	if err := c.BodyParser(&r); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	tx := DatabaseConnection.Where("user_id = ?", r.UserId).First(&b)

	if tx.Error != gorm.ErrRecordNotFound {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	} else if tx.Error == gorm.ErrRecordNotFound {
		return c.Status(http.StatusBadRequest).JSON(ErrUserIsNotBlacklisted)
	}

	tx = DatabaseConnection.Delete(&b)

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	var a []models.Avatar

	tx = DatabaseConnection.Where("avatar_author_id = ?", r.UserId).Find(&a)

	if tx.Error != gorm.ErrRecordNotFound {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	for _, avatar := range a {
		l := *avatar.GetLimitedAvatar()
		res, err := ReJsonClient.JSONSet(l.AvatarId, "$", l)

		if err != redis.Nil {
			fmt.Println(err)
		}

		if res.(string) != "OK" {
			fmt.Printf("Error adding avatar to search index: %s\n", l.AvatarId)
		}
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func BlacklistAvatarAuthor(c *fiber.Ctx) error {
	var r GenericUserRequest
	var b models.BlacklistedAuthor

	if err := c.BodyParser(&r); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	tx := DatabaseConnection.Where("user_id = ?", r.UserId).First(&b)

	if tx.Error == gorm.ErrRecordNotFound {
		tx = DatabaseConnection.Create(&models.BlacklistedAuthor{
			UserId: r.UserId,
		})

		if tx.Error != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}
	} else {
		return c.Status(http.StatusBadRequest).JSON(ErrUserAlreadyBlacklisted)
	}

	var a []models.Avatar

	tx = DatabaseConnection.Where("avatar_author_id = ?", r.UserId).Find(&a)

	if tx.Error == gorm.ErrRecordNotFound {
		return c.Status(http.StatusOK).JSON(fiber.Map{})
	} else if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	for _, avatar := range a {
		res, err := ReJsonClient.JSONDel(avatar.AvatarIdSha256, "$")

		if err != redis.Nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}

		if res.(string) != "OK" {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func BlacklistAvatar(c *fiber.Ctx) error {
	var a models.Avatar

	avatarId := c.Params("avatar_id")

	if avatarId == "" {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	tx := DatabaseConnection.Where("avatar_id = ?", avatarId).First(&a)

	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	} else if tx.Error == gorm.ErrRecordNotFound {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{})
	}

	a.AvatarPublic = false

	tx = DatabaseConnection.Save(&a)

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	res, err := ReJsonClient.JSONDel(a.AvatarIdSha256, "$")

	if err != redis.Nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	if res.(string) != "OK" {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func GetAdminAvatar(c *fiber.Ctx) error {
	var a models.Avatar

	avatarId := c.Params("avatar_id")

	if avatarId == "" {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	tx := DatabaseConnection.Where("avatar_id = ?", avatarId).First(&a)

	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	} else if tx.Error == gorm.ErrRecordNotFound {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{})
	}

	return c.Status(http.StatusOK).JSON(a)
}

func RebuildSearchIndex(c *fiber.Ctx) error {
	var a []models.Avatar

	_, err := RedisConnection.Get(ctx, "rebuild_search_index").Int()

	if err == nil {
		return c.Status(http.StatusBadRequest).JSON(ErrAlreadyRebuilding)
	}

	RedisConnection.FlushAll(ctx)
	cmd := RedisConnection.Do(ctx, "FT.CREATE",
		"avatarSearch", "ON", "JSON", "SCHEMA", "$.avatar_name", "AS", "avatar_name", "TEXT", "$.avatar_author_name ", "AS", "avatar_author_name", "TEXT")
	cmd.Val()

	go func() {
		RedisConnection.Set(ctx, "rebuild_search_index", 0, 0)

		var b []models.BlacklistedAuthor

		tx := DatabaseConnection.Find(&b)

		if tx.Error != nil {
			return
		}

		DatabaseConnection.Where("avatar_public = ?", "t").FindInBatches(&a, 1000, func(tx *gorm.DB, batch int) error {
			l := make([]models.LimitedAvatar, len(a))
			i := 0

			for _, result := range a {
				l[i] = *result.GetLimitedAvatar()
				i++
			}

			for _, v := range l {
				shouldIndex := true
				for _, b := range b {
					if v.AvatarAuthorId == b.UserId {
						shouldIndex = false
						break
					}
				}

				if !shouldIndex {
					continue
				}

				res, err := ReJsonClient.JSONSet(v.AvatarId, "$", v)

				if err != redis.Nil {
					fmt.Println(err)
				}

				if res.(string) != "OK" {
					fmt.Printf("Error adding avatar to search index: %s\n", v.AvatarId)
				}
			}

			fmt.Printf("Processed Batch: %d\n", batch)
			RedisConnection.Set(ctx, "rebuild_search_index", batch, 0)

			return nil
		})

		fmt.Println("Finished rebuilding search index")
		RedisConnection.Del(ctx, "rebuild_search_index")
	}()

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func RebuildSearchIndexStatus(c *fiber.Ctx) error {
	batch, err := RedisConnection.Get(ctx, "rebuild_search_index").Result()

	if err == redis.Nil {
		return c.Status(http.StatusNoContent).JSON(fiber.Map{})
	} else if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	batchInt, err := strconv.Atoi(batch)

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"batch": batchInt})
}
