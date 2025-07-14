package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	DBSource          string `mapstructure:"DB_SOURCE"`
	HTTPServerAddress string `mapstructure:"HTTP_SERVER_ADDRESS"`

	// RabbitMQ Configuration
	RabbitMQURL             string `mapstructure:"RABBITMQ_URL"`
	RabbitMQExchange        string `mapstructure:"RABBITMQ_EXCHANGE"`
	RabbitMQQueueEmail      string `mapstructure:"RABBITMQ_QUEUE_EMAIL"`
	RabbitMQRoutingKeyEmail string `mapstructure:"RABBITMQ_ROUTING_KEY_EMAIL"`

	// SMTP Configuration
	SMTPHost string `mapstructure:"SMTP_HOST"`
	SMTPPort int    `mapstructure:"SMTP_PORT"`
	SMTPFrom string `mapstructure:"SMTP_FROM"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigType("env")
	viper.SetConfigFile(".env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&config)
	return
}
