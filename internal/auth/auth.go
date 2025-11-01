package auth

import (
	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error) {
	str, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	return str, err
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	return match, err
}
