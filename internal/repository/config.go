package repository

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config for DB.
type Config struct {
	Port     int    `envconfig:"POSTGRES_PORT" default:"5432"`
	Host     string `envconfig:"POSTGRES_HOST" default:"localhost"`
	User     string `envconfig:"POSTGRES_USER" default:"postgres"`
	Password string `envconfig:"POSTGRES_PASSWORD" default:"postgres"`
	Database string `envconfig:"POSTGRES_DB" default:"postgres"`
	SSLMode  string `envconfig:"POSTGRES_SSLMODE" default:"disable"`
}

// URL returns connection string.
func (c Config) URL() string {
	return fmt.Sprintf(
		"user=%v password=%v host=%v port=%v dbname=%v sslmode=%v",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode,
	)
}

// LoadConfig loads envs.
func LoadConfig() (Config, error) {
	c := Config{}
	if err := envconfig.Process("", &c); err != nil {
		return c, err
	}
	return c, nil
}

// MustConfig loads envs.
// Panics in case of error.
func MustConfig(c Config, err error) Config {
	if err != nil {
		panic(err)
	}
	return c
}
