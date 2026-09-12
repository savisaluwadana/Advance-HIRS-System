package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID         string `json:"uid"`
	Email          string `json:"email"`
	DisplayName    string `json:"name"`
	OrganizationID string `json:"org"`
	Organization   string `json:"org_name"`
	Role           string `json:"role"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) (*Manager, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &Manager{secret: []byte(secret), ttl: ttl}, nil
}

func (m *Manager) Issue(userID, email, displayName, orgID, orgName, role string) (string, time.Time, error) {
	now := time.Now().UTC()
	expires := now.Add(m.ttl)
	claims := Claims{
		UserID:         userID,
		Email:          email,
		DisplayName:    displayName,
		OrganizationID: orgID,
		Organization:   orgName,
		Role:           role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "advance-hris",
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	return signed, expires, err
}

func (m *Manager) Parse(raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.secret, nil
	}, jwt.WithIssuer("advance-hris"), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid access token")
	}
	return claims, nil
}

func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password must be at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
