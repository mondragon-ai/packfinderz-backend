package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/config"
	"golang.org/x/crypto/argon2"
)

var tempPasswordCharset = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

// ErrInvalidHash signals a malformed Argon2id hash string.
var ErrInvalidHash = fmt.Errorf("invalid argon2id hash")

// ArgonParams captures the Argon2id parameters we embed into each hash string.
type ArgonParams struct {
	Memory      uint32
	Time        uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// HashPassword returns a formatted Argon2id hash for the provided password.
func HashPassword(password string, cfg config.PasswordConfig) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	params := paramsFromConfig(cfg)
	salt := make([]byte, params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Parallelism, params.KeyLen)

	encSalt := base64.RawStdEncoding.EncodeToString(salt)
	encHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", params.Memory, params.Time, params.Parallelism, encSalt, encHash), nil
}

// VerifyPassword returns true when the password matches the encoded hash.
func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, hash, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}

	computed := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Parallelism, params.KeyLen)

	if subtle.ConstantTimeCompare(hash, computed) == 1 {
		return true, nil
	}
	return false, nil
}

func paramsFromConfig(cfg config.PasswordConfig) ArgonParams {
	threads := clampInt(cfg.ArgonParallelism, 1, 255)
	return ArgonParams{
		Memory:      clampUint32(cfg.ArgonMemoryKB, 8, 512*1024),
		Time:        clampUint32(cfg.ArgonTime, 1, 10),
		Parallelism: uint8(threads),
		SaltLen:     clampUint32(cfg.ArgonSaltLen, 8, 64),
		KeyLen:      clampUint32(cfg.ArgonKeyLen, 16, 64),
	}
}

func decodeHash(encoded string) (ArgonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return ArgonParams{}, nil, nil, ErrInvalidHash
	}

	paramsPart := parts[3]
	var params ArgonParams
	for _, token := range strings.Split(paramsPart, ",") {
		keyValue := strings.SplitN(token, "=", 2)
		if len(keyValue) != 2 {
			return ArgonParams{}, nil, nil, ErrInvalidHash
		}
		key, value := keyValue[0], keyValue[1]
		switch key {
		case "m":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				params.Memory = uint32(v)
			} else {
				return ArgonParams{}, nil, nil, ErrInvalidHash
			}
		case "t":
			if v, err := strconv.ParseUint(value, 10, 32); err == nil {
				params.Time = uint32(v)
			} else {
				return ArgonParams{}, nil, nil, ErrInvalidHash
			}
		case "p":
			if v, err := strconv.ParseUint(value, 10, 8); err == nil {
				params.Parallelism = uint8(v)
			} else {
				return ArgonParams{}, nil, nil, ErrInvalidHash
			}
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ArgonParams{}, nil, nil, ErrInvalidHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ArgonParams{}, nil, nil, ErrInvalidHash
	}

	params.SaltLen = uint32(len(salt))
	params.KeyLen = uint32(len(hash))

	return params, salt, hash, nil
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampUint32(value, min, max int) uint32 {
	return uint32(clampInt(value, min, max))
}

// GenerateTempPassword produces a random string suitable for temporary credentials.
func GenerateTempPassword(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be positive")
	}

	result := make([]rune, length)
	for i := 0; i < length; i++ {
		idx, err := randInt(len(tempPasswordCharset))
		if err != nil {
			return "", err
		}
		result[i] = tempPasswordCharset[idx]
	}
	return string(result), nil
}

func randInt(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("invalid max %d", max)
	}
	var buff = make([]byte, 1)
	if _, err := rand.Read(buff); err != nil {
		return 0, err
	}
	return int(buff[0]) % max, nil
}
