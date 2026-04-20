package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig
	AWS      AWSConfig
	Midtrans MidtransConfig
	Fonnte   FonnteConfig
}

type FonnteConfig struct {
	Token string
}

type AppConfig struct {
	Port string
	Env  string // "development" | "production"
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  int // dalam menit
	RefreshTokenTTL int // dalam hari
}

type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	S3Bucket        string
}

type MidtransConfig struct {
	ServerKey  string
	ClientKey  string
	Production bool
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// set default values
	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("DB_SSLMODE", "disable")
	viper.SetDefault("JWT_ACCESS_TTL", 60)    // 1 jam
	viper.SetDefault("JWT_REFRESH_TTL", 7)    // 7 hari
	viper.SetDefault("MIDTRANS_PRODUCTION", false)

	if err := viper.ReadInConfig(); err != nil {
		// .env tidak wajib ada, fallback ke OS env
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	cfg := &Config{
		App: AppConfig{
			Port: viper.GetString("APP_PORT"),
			Env:  viper.GetString("APP_ENV"),
		},
		Database: DatabaseConfig{
			Host:     viper.GetString("DB_HOST"),
			Port:     viper.GetString("DB_PORT"),
			User:     viper.GetString("DB_USER"),
			Password: viper.GetString("DB_PASSWORD"),
			Name:     viper.GetString("DB_NAME"),
			SSLMode:  viper.GetString("DB_SSLMODE"),
		},
		JWT: JWTConfig{
			SecretKey:       viper.GetString("JWT_SECRET"),
			AccessTokenTTL:  viper.GetInt("JWT_ACCESS_TTL"),
			RefreshTokenTTL: viper.GetInt("JWT_REFRESH_TTL"),
		},
		AWS: AWSConfig{
			Region:          viper.GetString("AWS_REGION"),
			AccessKeyID:     viper.GetString("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: viper.GetString("AWS_SECRET_ACCESS_KEY"),
			S3Bucket:        viper.GetString("AWS_S3_BUCKET"),
		},
		Midtrans: MidtransConfig{
			ServerKey:  viper.GetString("MIDTRANS_SERVER_KEY"),
			ClientKey:  viper.GetString("MIDTRANS_CLIENT_KEY"),
			Production: viper.GetBool("MIDTRANS_PRODUCTION"),
		},
		Fonnte: FonnteConfig{
			Token: viper.GetString("FONNTE_TOKEN"),
		},
	}

	return cfg, nil
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s pool_max_conns=10",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}
