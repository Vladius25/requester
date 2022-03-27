package api

import (
	"github.com/kelseyhightower/envconfig"
	"strings"
)

// Config for API.
type Config struct {
	MountPrefix   string `envconfig:"MOUNT_PREFIX" default:"/api/v1"`
	ListenAddress string `envconfig:"LISTEN_ADDR" default:":3000"`
	TaskQueue     string `envconfig:"TASK_QUEUE" default:"task-queue"`
}

// LoadConfig loads envs.
func LoadConfig() (Config, error) {
	c := Config{}
	if err := envconfig.Process("", &c); err != nil {
		return c, err
	}
	c.MountPrefix = strings.TrimSuffix(c.MountPrefix, "/")
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
