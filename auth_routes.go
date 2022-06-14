package main

import (
	"emmApi/models"
	"github.com/alexedwards/argon2id"
	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Argon2IdParams these are calibrated to the current hardware
var Argon2IdParams = argon2id.Params{
	Memory:      125000,
	Iterations:  1,
	Parallelism: 12,
	SaltLength:  16,
	KeyLength:   32,
}

var UserIdRegex = regexp.MustCompile(`^(usr_[0-9a-f]{8}-[0-9a-f]{4}-[0-5][0-9a-f]{3}-[089ab][0-9a-f]{3}-[0-9a-f]{12}|[a-zA-Z0-9]{1,10})$`)
var PasswordRegex = regexp.MustCompile(`^\d+$`)

var ErrPersistentTokenNotProvided = fiber.Map{"error": "Persistent token not provided."}
var ErrPersistentTokenInvalid = fiber.Map{"error": "Persistent token invalid."}
var ErrUserIdDoesNotMatchPattern = fiber.Map{"error": "User ID does not match pattern."}
var ErrPasswordDoesNotMatchPattern = fiber.Map{"error": "Password does not match required pattern."}
var ErrInvalidPassword = fiber.Map{"error": "Invalid password."}
var ErrPasswordResetRequired = fiber.Map{"error": "Password reset required."}

var LetterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

func authRoutes(router fiber.Router) {
	router.Post("/auth", doAuth)
	router.Post("/auth/reset", doReset)
	router.Patch("/auth", JwtRequired, EnforceModeration, doRefreshToken)
	router.Delete("/auth", JwtRequired, EnforceModeration, doRevokeToken)
}

func doAuth(c *fiber.Ctx) error {
	var a AuthenticationRequest
	var u models.User

	if err := c.BodyParser(&a); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	if !UserIdRegex.Match([]byte(a.UserId)) {
		return c.Status(http.StatusBadRequest).JSON(ErrUserIdDoesNotMatchPattern)
	}

	if !a.NeedPersistentToken && a.PersistentToken == "" {
		return c.Status(http.StatusBadRequest).JSON(ErrPersistentTokenNotProvided)
	}

	if a.Password != "" && !a.NeedPersistentToken {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidPassword)
	}

	if a.Password != "" && !PasswordRegex.Match([]byte(a.Password)) {
		return c.Status(http.StatusBadRequest).JSON(ErrPasswordDoesNotMatchPattern)
	}

	if a.UserId == a.Password {
		return c.Status(http.StatusUnauthorized).JSON(ErrInvalidPassword)
	}

	tx := DatabaseConnection.Where("user_id = ?", a.UserId).First(&u)

	if tx.Error == gorm.ErrRecordNotFound {
		hash, err := argon2id.CreateHash(a.Password, &Argon2IdParams)

		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}

		u = models.User{
			UserId:           a.UserId,
			UserKnownAliases: pq.StringArray{a.DisplayName},
			UserPin:          hash,
			LastSeen:         time.Now(),
			CreatedAt:        time.Now(),
		}
	}

	// Password is compromised, so we need to trigger a reset
	if strings.HasPrefix(u.UserPin, "$2b") {
		err := bcrypt.CompareHashAndPassword([]byte(u.UserPin), []byte(a.Password))

		if err == nil {
			return c.Status(http.StatusUpgradeRequired).JSON(ErrPasswordResetRequired)
		} else {
			err = bcrypt.CompareHashAndPassword([]byte(u.UserId), []byte(a.Password))

			if err == nil {
				return c.Status(http.StatusUpgradeRequired).JSON(ErrPasswordResetRequired)
			}

			return c.Status(http.StatusUnauthorized).JSON(ErrInvalidPassword)
		}
	}

	if u.UserPin == "" {
		hash, err := argon2id.CreateHash(a.Password, &Argon2IdParams)

		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}

		u.UserPin = hash
	}

	if a.PersistentToken == "" {
		match, err := argon2id.ComparePasswordAndHash(a.Password, u.UserPin)

		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}

		if !match {
			return c.Status(http.StatusUnauthorized).JSON(ErrInvalidPassword)
		}
	} else {
		var p models.PersistentToken

		tx := DatabaseConnection.Where("user_id = ? AND token = ?", a.UserId, a.PersistentToken).First(&p)

		if tx.Error != nil {
			return c.Status(http.StatusUnauthorized).JSON(ErrPersistentTokenInvalid)
		}
	}

	u.LastSeen = time.Now()

	knownAlias := false

	for _, v := range u.UserKnownAliases {
		if v == a.DisplayName {
			knownAlias = true
			break
		}
	}

	if !knownAlias {
		u.UserKnownAliases = append(pq.StringArray{a.DisplayName}, u.UserKnownAliases...)
	}

	DatabaseConnection.Save(&u)

	ban := GetBan(u.UserId, c.IP())

	if ban != nil {
		return c.Status(http.StatusForbidden).JSON(ban)
	}

	token, err := IssueToken(u.UserId, c.IP())

	if IsExpired(&u) {
		QueueUserCheck(u.UserId)
	}

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	persistentToken := ""

	if a.NeedPersistentToken {
		persistentToken = GeneratePersistentToken()

		t := models.PersistentToken{
			UserId: u.UserId,
			Token:  persistentToken,
		}

		tx := DatabaseConnection.Save(&t)

		if tx.Error != nil {
			return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
		}
	}

	return c.Status(http.StatusOK).JSON(AuthenticationResponse{
		Token:           token,
		PersistentToken: persistentToken,
	})
}

func GeneratePersistentToken() string {
	b := make([]byte, 128)

	for i := range b {
		b[i] = LetterBytes[rand.Intn(len(LetterBytes))]
	}

	return string(b)
}

func doReset(c *fiber.Ctx) error {
	var p PasswordResetRequest
	var u models.User

	if err := c.BodyParser(&p); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrInvalidRequestBody)
	}

	if !PasswordRegex.Match([]byte(p.Password)) {
		return c.Status(http.StatusBadRequest).JSON(ErrPasswordDoesNotMatchPattern)
	}

	tx := DatabaseConnection.Where("user_id = ?", p.UserId).First(&u)

	if tx.Error != nil {
		return c.Status(http.StatusUnauthorized).JSON(ErrInvalidPassword)
	}

	err := bcrypt.CompareHashAndPassword([]byte(u.UserPin), []byte(p.Password))

	if err != nil {
		err = bcrypt.CompareHashAndPassword([]byte(u.UserId), []byte(p.Password))

		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(ErrInvalidPassword)
		}
	}

	hash, err := argon2id.CreateHash(p.Password, &Argon2IdParams)

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	u.UserPin = hash
	DatabaseConnection.Save(&u)

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}

func doRefreshToken(c *fiber.Ctx) error {
	var u models.User

	userId := c.Locals("userId").(string)

	tx := DatabaseConnection.Where("user_id = ?", userId).First(&u)

	if tx.Error != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{})
	}

	token, err := IssueToken(userId, c.IP())

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	u.LastSeen = time.Now()
	DatabaseConnection.Save(&u)

	return c.Status(http.StatusOK).JSON(AuthenticationResponse{
		Token:           token,
		PersistentToken: "",
	})
}

func doRevokeToken(c *fiber.Ctx) error {
	tx := DatabaseConnection.Delete(&models.PersistentToken{}, "user_id = ?", c.Locals("userId").(string))

	if tx.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrInternalServerError)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{})
}
