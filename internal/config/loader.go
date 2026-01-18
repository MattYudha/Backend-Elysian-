package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var validate = validator.New()

// load reads configuration from multiple sources and returns a validated Config
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables and config files")
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	// setup Viper
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	// read default config
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read default config: %w", err)
	}

	// merge environment-specific config
	v.SetConfigName(fmt.Sprintf("config.%s", env))
	if err := v.MergeInConfig(); err != nil {
		log.Printf("No environment-specific config found for '%s', using defaults", env)
	}

	// enable environment variable override
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// unmarshal config into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// overide with environment variables
	overrideWithEnv(&cfg)

	// validate configuration
	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// additional custom validation
	if err := validateCustomRules(&cfg); err != nil {
		return nil, fmt.Errorf("custom validation failed: %w", err)
	}

	return &cfg, nil
}

// overrideWithEnv overrides config values with environment variables
func overrideWithEnv(cfg *Config) {
	// Server
	if v := os.Getenv("PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("ENV"); v != "" {
		cfg.Server.Environment = v
	}

	// Database
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		cfg.Database.Port = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("DB_SSL_MODE"); v != "" {
		cfg.Database.SSLMode = v
	}

	// Redis
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		cfg.Redis.Port = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	// JWT
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}

	// RabbitMQ
	if v := os.Getenv("RABBITMQ_URL"); v != "" {
		cfg.RabbitMQ.URL = v
	}

	// Storage
	if v := os.Getenv("S3_ENDPOINT"); v != "" {
		cfg.Storage.Endpoint = v
	}
	if v := os.Getenv("S3_ACCESS_KEY"); v != "" {
		cfg.Storage.AccessKey = v
	}
	if v := os.Getenv("S3_SECRET_KEY"); v != "" {
		cfg.Storage.SecretKey = v
	}
	if v := os.Getenv("S3_BUCKET"); v != "" {
		cfg.Storage.Bucket = v
	}

	// ML Service
	if v := os.Getenv("ML_SERVICE_URL"); v != "" {
		cfg.ML.ServiceURL = v
	}
}

// MaskSensitive returns a copy of the config with sensitive values masked
func (c *Config) MaskSensitive() *Config {
	masked := *c
	masked.Database.Password = "***MASKED***"
	masked.Redis.Password = "***MASKED***"
	masked.JWT.Secret = "***MASKED***"
	masked.Storage.AccessKey = "***MASKED***"
	masked.Storage.SecretKey = "***MASKED***"
	return &masked
}

// GetDatabaseDSN returns the database connection string
func (c *Config) GetDatabaseDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Name,
		c.Database.SSLMode,
	)
}

// GetRedisDSN returns the Redis connection string
func (c *Config) GetRedisDSN() string {
	if c.Redis.Password != "" {
		return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
	}
	return fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port)
}

// IsDevelopment returns true if environment is development
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

// IsProduction returns true if environment is production
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}
