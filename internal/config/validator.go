package config

import (
	"fmt"
	"strconv"
)

// validateCustomRules performs additional validation beyond struct tags
func validateCustomRules(cfg *Config) error {
	// Validate port is a valid number
	if port, err := strconv.Atoi(cfg.Server.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid server port '%s', must be between 1-65535", cfg.Server.Port)
	}

	// Validate database port
	if port, err := strconv.Atoi(cfg.Database.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid database port '%s', must be between 1-65535", cfg.Database.Port)
	}

	// Validate Redis port
	if port, err := strconv.Atoi(cfg.Redis.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid redis port '%s', must be between 1-65535", cfg.Redis.Port)
	}

	// Validate JWT secret length in production
	if cfg.IsProduction() && len(cfg.JWT.Secret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters in production, got %d", len(cfg.JWT.Secret))
	}

	// Validate timeout values are positive
	if cfg.Server.ReadTimeout <= 0 {
		return fmt.Errorf("server read_timeout must be positive, got %v", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout <= 0 {
		return fmt.Errorf("server write_timeout must be positive, got %v", cfg.Server.WriteTimeout)
	}

	// Validate database pool settings
	if cfg.Database.MaxOpenConns < cfg.Database.MaxIdleConns {
		return fmt.Errorf("database max_open_conns (%d) must be >= max_idle_conns (%d)",
			cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns)
	}

	return nil
}
