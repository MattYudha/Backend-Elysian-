package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

type PasswordService struct{}

func NewPasswordService() *PasswordService {
	return &PasswordService{}
}

// HashPassword generates a secure Argon2id hash using enterprise parameters
func (s *PasswordService) HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password cannot be empty")
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	// Mandated enterprise parameters
	iterations := uint32(3)
	memory := uint32(64 * 1024) // 64MB (65536 KB)
	parallelism := uint8(4)
	keyLength := uint32(32)

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)

	// Format into standard serialized Argon2id string
	saltBase64 := base64.RawStdEncoding.EncodeToString(salt)
	hashBase64 := base64.RawStdEncoding.EncodeToString(hash)

	formatted := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, iterations, parallelism, saltBase64, hashBase64)

	return formatted, nil
}

// ComparePassword verifies the raw password against the stored hash.
// Gracefully falls back to bcrypt if it detects a legacy password hash.
func (s *PasswordService) ComparePassword(hashedPassword, password string) error {
	if hashedPassword == "" || password == "" {
		return errors.New("hashed password and raw password cannot be empty")
	}

	// Backward Compatibility: Fallback to bcrypt if it doesn't match the Argon2id format
	if !strings.HasPrefix(hashedPassword, "$argon2id$") {
		err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
		if err != nil {
			if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
				return errors.New("invalid password")
			}
			return fmt.Errorf("password verification failed: %w", err)
		}
		return nil
	}

	// Parse Argon2id elements
	parts := strings.Split(hashedPassword, "$")
	if len(parts) < 6 {
		return errors.New("invalid argon2id hash format")
	}

	var memory, iterations uint32
	var parallelism uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return fmt.Errorf("failed to parse argon2id parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("failed to decode argon2id salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("failed to decode argon2id hash: %w", err)
	}

	keyLength := uint32(len(expectedHash))
	actualHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(expectedHash, actualHash) != 1 {
		return errors.New("invalid password")
	}

	return nil
}
