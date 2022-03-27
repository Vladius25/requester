package requester

import (
	"github.com/kelseyhightower/envconfig"
)

// Config for Requester.
type Config struct {
	Workers   int    `envconfig:"WORKERS" default:"3"`
	TaskQueue string `envconfig:"TASK_QUEUE" default:"task-queue"`
}

// LoadConfig loads envs.
func LoadConfig() (Config, error) {
	c := Config{}
	return c, envconfig.Process("", &c)
}

// MustConfig loads envs.
// Panics in case of error.
func MustConfig(c Config, err error) Config {
	if err != nil {
		panic(err)
	}
	return c
}
