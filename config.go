package main

var ServiceConfig Config

type Config struct {
	AdminSecret  string             `json:"admin_secret"`
	Database     DatabaseConfig     `json:"database"`
	Redis        RedisConfig        `json:"redis"`
	Jwt          JwtConfig          `json:"jwt"`
	CheckService CheckServiceConfig `json:"check_service"`
}

type DatabaseConfig struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"username"`
	Password           string `json:"password"`
	Database           string `json:"database"`
	MaxIdleConnections int    `json:"max_idle_connections"`
	MaxOpenConnections int    `json:"max_open_connections"`
}

type RedisConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type JwtConfig struct {
	Secret       string `json:"secret"`
	ReissueDelay int    `json:"reissue_delay"`
	Timeout      int    `json:"timeout"`
}

type CheckServiceConfig struct {
	CheckEnabled bool   `json:"check_enabled"`
	CheckUrl     string `json:"check_url"`
}
