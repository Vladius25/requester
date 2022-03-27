package queue

import (
	"github.com/kelseyhightower/envconfig"
	"time"
)

type SQSParams struct {
	VisibilityTimeout  time.Duration
	MaxMessageAttempts int
}

// Config for SQS.
type Config struct {
	SQSParams
	URL       string `envconfig:"AWS_SQS_ENDPOINT_URL" required:"true"`
	KeyID     string `envconfig:"AWS_ACCESS_KEY_ID" required:"true"`
	SecretKey string `envconfig:"AWS_SECRET_ACCESS_KEY" required:"true"`
	Region    string `envconfig:"AWS_SQS_REGION" default:"ru-central1"`
	Debug     bool   `envconfig:"DEBUG" default:"false"`
}

// LoadConfig loads envs.
func LoadConfig(params SQSParams) (Config, error) {
	c := Config{SQSParams: params}
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
