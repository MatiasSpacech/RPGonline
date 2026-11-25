// Package auth implementa el hashing de contraseñas con Argon2id.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// Parámetros recomendados (interactivos): 64 MB, 2 iteraciones, 1 hilo.
	argonMemory  = 64 * 1024
	argonTime    = 2
	argonThreads = 1
	saltLen      = 16
	keyLen       = 32
)

// ErrInvalidHash indica un hash con formato desconocido.
var ErrInvalidHash = errors.New("hash de contraseña inválido")

// HashPassword genera un hash Argon2id en formato PHC con salt aleatorio.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: no se pudo generar el salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, keyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword compara una contraseña con un hash PHC en tiempo constante.
func VerifyPassword(password, encoded string) bool {
	key, salt, memory, iterations, threads, err := decodeHash(encoded)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(got, key) == 1
}

func decodeHash(encoded string) (key, salt []byte, memory, iterations uint32, threads uint8, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}
	if len(salt) == 0 || len(key) == 0 {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}
	return key, salt, memory, iterations, threads, nil
}
