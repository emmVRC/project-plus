package main

import (
	"emmApi/models"
	"github.com/gofiber/fiber/v2"
	"net/http"
	"time"
)

func EnforceModeration(c *fiber.Ctx) error {
	userId := c.Locals("userId").(string)
	ban := GetBan(userId, c.IP())

	if ban != nil {
		return c.Status(http.StatusForbidden).JSON(ban)
	}

	return c.Next()
}

func GetBan(userId string, ipAddress string) *models.Ban {
	var ban models.Ban
	err := DatabaseConnection.Where("ban_user_id = ? OR ip_address = ?", userId, ipAddress).First(&ban).Error

	if err != nil {
		return nil
	}

	if ban.BanExpires.Unix() > time.UnixMilli(0).Unix() && ban.BanExpires.Unix() < time.Now().Unix() {
		return nil
	}

	return &ban
}
