package secure

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const authTokenUser = "desktop-access-token"

func LoadAccessToken() (string, error) {
	token, err := keyring.Get(keyringService, authTokenUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return token, err
}

func SaveAccessToken(token string) error {
	if token == "" {
		return ClearAccessToken()
	}
	return keyring.Set(keyringService, authTokenUser, token)
}

func ClearAccessToken() error {
	err := keyring.Delete(keyringService, authTokenUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
