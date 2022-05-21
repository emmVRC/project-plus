package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"net/http"
	"strings"
	"time"
)

var ErrMissingBearerToken = fiber.Map{"error": "Missing bearer token."}
var ErrInvalidBearerToken = fiber.Map{"error": "Invalid bearer token provided."}
var ErrIpMismatch = fiber.Map{"error": "Connecting IP does not match the provided ip."}

type AssignedUser struct {
	UserID    string `json:"user_id"`
	IpAddress string `json:"ip_address"`
	jwt.StandardClaims
}

func IssueToken(userId string, ipAddress string) (string, error) {
	claims := AssignedUser{
		UserID:    userId,
		IpAddress: ipAddress,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Duration(ServiceConfig.Jwt.Timeout) * time.Second).Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(ServiceConfig.Jwt.Secret))
}

func ValidateToken(providedToken string, ipAddress string) (string, fiber.Map) {
	claims := AssignedUser{}
	token, err := jwt.ParseWithClaims(providedToken, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(ServiceConfig.Jwt.Secret), nil
	})

	if err != nil {
		return "", ErrInvalidBearerToken
	}

	if !token.Valid {
		return "", ErrInvalidBearerToken
	}

	if ipAddress != claims.IpAddress {
		return "", ErrIpMismatch
	}

	return claims.UserID, nil
}

func JwtRequired(c *fiber.Ctx) error {
	authorizationHeader := c.Get("Authorization")

	if !strings.HasPrefix(authorizationHeader, "Bearer ") {
		return c.Status(http.StatusUnauthorized).
			JSON(ErrMissingBearerToken)
	}

	authorizationHeader = strings.TrimPrefix(authorizationHeader, "Bearer ")
	userId, err := ValidateToken(authorizationHeader, c.IP())

	if err != nil {
		return c.Status(http.StatusUnauthorized).
			JSON(err)
	}

	c.Locals("userId", userId)
	return c.Next()
}
