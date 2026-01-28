package config

import "time"

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Storage  StorageConfig  `mapstructure:"storage"`
	ML       MLConfig       `mapstructure:"ml"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Upload   UploadConfig   `mapstructure:"upload"`
}

type ServerConfig struct {
	Port                    string        `mapstructure:"port" validate:"required"`
	Host                    string        `mapstructure:"host"`
	Environment             string        `mapstructure:"environment" validate:"required,oneof=development staging production"`
	ReadTimeout             time.Duration `mapstructure:"read_timeout"`
	WriteTimeout            time.Duration `mapstructure:"write_timeout"`
	IdleTimeout             time.Duration `mapstructure:"idle_timeout"`
	GracefulShutdownTimeout time.Duration `mapstructure:"graceful_shutdown_timeout"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host" validate:"required"`
	Port            string        `mapstructure:"port" validate:"required"`
	User            string        `mapstructure:"user" validate:"required"`
	Password        string        `mapstructure:"password" validate:"required"`
	Name            string        `mapstructure:"name" validate:"required"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns" validate:"min=1"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns" validate:"min=1"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host" validate:"required"`
	Port     string `mapstructure:"port" validate:"required"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size" validate:"min=1"`
}

type JWTConfig struct {
	Secret             string        `mapstructure:"secret" validate:"required,min=32"`
	AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry" validate:"required"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry" validate:"required"`
	Issuer             string        `mapstructure:"issuer"`
}

type RabbitMQConfig struct {
	URL         string `mapstructure:"url"`
	QueueName   string `mapstructure:"queue_name"`
	WorkerCount int    `mapstructure:"worker_count" validate:"min=1"`
}

type StorageConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

type MLConfig struct {
	ServiceURL string        `mapstructure:"service_url"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RetryCount int           `mapstructure:"retry_count" validate:"min=0"`
	RetryDelay time.Duration `mapstructure:"retry_delay"`
}

type SecurityConfig struct {
	RateLimitRequestsPerMinute int      `mapstructure:"rate_limit_requests_per_minute" validate:"min=1"`
	RateLimitBurst             int      `mapstructure:"rate_limit_burst" validate:"min=1"`
	CORSAllowedOrigins         []string `mapstructure:"cors_allowed_origins"`
	CORSAllowedMethods         []string `mapstructure:"cors_allowed_methods"`
	CORSAllowedHeaders         []string `mapstructure:"cors_allowed_headers"`
	CORSAllowCredentials       bool     `mapstructure:"cors_allow_credentials"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	Format string `mapstructure:"format" validate:"required,oneof=json text"`
	Output string `mapstructure:"output" validate:"required,oneof=stdout stderr file"`
}

type UploadConfig struct {
	MaxFileSize      int64    `mapstructure:"max_file_size" validate:"min=1"`
	AllowedFileTypes []string `mapstructure:"allowed_file_types"`
}
