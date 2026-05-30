package gateway

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/leyl1ne/ApiGateway/internal/auth/jwt"
	"github.com/leyl1ne/ApiGateway/internal/http"
	"github.com/leyl1ne/ApiGateway/internal/logger"
)

type Config struct {
	Server   http.Config    `yaml:"server"`
	CORS     CORSConfig     `yaml:"cors"`
	JWT      jwt.Config     `yaml:"jwt"`
	Services ServicesConfig `yaml:"services"`
	Logger   logger.Config  `yaml:"logger"`
}

type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

type ServicesConfig struct {
	UserService ServiceConfig `yaml:"user_service"`
	// Добавлять новые сервисы здесь по мере роста проекта:
	// OrderService   ServiceConfig `yaml:"order_service"`
	// ReportService  ServiceConfig `yaml:"report_service"`
}

type ServiceConfig struct {
	URL string `yaml:"url"`
}

func (c *Config) Validate() error {
	if c.Server.HTTP.Port <= 0 || c.Server.HTTP.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.HTTP.Port)
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt secret is required")
	}
	if c.Services.UserService.URL == "" {
		return fmt.Errorf("user_service url is required")
	}
	return nil
}

func Load() (*Config, error) {
	const op = "config.gateway.Load"

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		return nil, fmt.Errorf("%s: CONFIG_PATH environment variable is not set", op)
	}

	// Проверяем существование конфиг-файла
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("%s: error opening config file: %w", op, err)
	}

	var cfg Config

	// Читаем конфиг-файл и заполняем нашу структуру
	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: error reading config file: %w", op, err)
	}

	return &cfg, nil
}
