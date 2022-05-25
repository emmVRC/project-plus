package main

import (
	"emmApi/models"
	"fmt"
	"github.com/RediSearch/redisearch-go/redisearch"
	goredis "github.com/go-redis/redis/v8"
	"github.com/nitishm/go-rejson/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

var DatabaseConnection *gorm.DB
var RedisConnection *goredis.Client
var ReJsonClient *rejson.Handler
var RediSearchClient *redisearch.Client

func SetupDatabaseConnection() {
	databaseConfig := ServiceConfig.Database

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Etc/UTC",
		databaseConfig.Host,
		databaseConfig.User,
		databaseConfig.Password,
		databaseConfig.Database,
		databaseConfig.Port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
		//Logger: gormLogger.Default.LogMode(gormLogger.Info),
	})

	if err != nil {
		panic(err)
	}

	sqlDb, err := db.DB()

	if err != nil {
		panic(err)
	}

	sqlDb.SetMaxIdleConns(databaseConfig.MaxIdleConnections)
	sqlDb.SetMaxOpenConns(databaseConfig.MaxOpenConnections)

	err = db.AutoMigrate(&models.Avatar{})
	if err != nil {
		fmt.Println(err)
	}

	err = db.AutoMigrate(&models.User{})
	if err != nil {
		fmt.Println(err)
	}

	err = db.AutoMigrate(&models.BlacklistedAuthor{})
	if err != nil {
		fmt.Println(err)
	}

	err = db.AutoMigrate(&models.PersistentToken{})
	if err != nil {
		fmt.Println(err)
	}

	err = db.AutoMigrate(&models.Ban{})
	if err != nil {
		fmt.Println(err)
	}

	err = db.AutoMigrate(&models.AvatarFavorite{})
	if err != nil {
		fmt.Println(err)
	}

	DatabaseConnection = db
}

func SetupRedisConnection() {
	redisConfig := ServiceConfig.Redis
	host := fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port)

	rh := rejson.NewReJSONHandler()
	client := goredis.NewClient(&goredis.Options{Addr: host})
	rs := redisearch.NewClient(host, "avatarSearch")

	rh.SetGoRedisClient(client)

	RedisConnection = client
	ReJsonClient = rh
	RediSearchClient = rs
}
