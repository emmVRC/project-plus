package main

import (
	"context"
	"emmApi/models"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
)

var ctx = context.Background()

var ErrAlreadyRebuilding = fiber.Map{"error": "Search index rebuild is already running. Please wait."}

func adminRoutes(router fiber.Router) {
	router.Post("/admin/rebuild_search_index", EnforceAdminSecret, RebuildSearchIndex)
	router.Get("/admin/rebuild_search_index/status", EnforceAdminSecret, RebuildSearchIndexStatus)
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

		DatabaseConnection.Where("avatar_public = ?", "t").FindInBatches(&a, 1000, func(tx *gorm.DB, batch int) error {
			l := make([]models.LimitedAvatar, len(a))
			i := 0

			for _, result := range a {
				l[i] = *result.GetLimitedAvatar()
				i++
			}

			for _, v := range l {
				res, err := ReJsonClient.JSONSet(v.AvatarId, "$", v)

				if err != nil {
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
