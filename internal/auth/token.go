// Package auth отвечает за хеширование паролей и выпуск токенов доступа.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken возвращается, когда токен повреждён, просрочен
// или подписан другим ключом.
var (
	ErrInvalidToken = errors.New("invalid token")
)

// CookieName — имя cookie, в которой передаётся токен аутентификации.
const CookieName = "token"

// Claims — полезная нагрузка токена: стандартные поля JWT и идентификатор пользователя.
type Claims struct {
	jwt.RegisteredClaims
	UserID int
}

// TokenManager выпускает и проверяет JWT-токены, подписанные HS256.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenManager создаёт TokenManager, подписывающий токены ключом secret
// и выпускающий их со сроком жизни ttl.
func NewTokenManager(secret []byte, ttl time.Duration) *TokenManager {
	return &TokenManager{
		secret: secret,
		ttl:    ttl,
	}
}

// Issue выпускает подписанный токен для пользователя userID.
func (m *TokenManager) Issue(userID int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl))},
		UserID:           userID,
	})
	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return tokenString, nil
}

// Parse проверяет подпись и срок действия токена и возвращает идентификатор
// пользователя. Для невалидного токена возвращает ошибку, обёрнутую ErrInvalidToken.
func (m *TokenManager) Parse(tokenString string) (int, error) {
	claims := Claims{}

	_, err := jwt.ParseWithClaims(tokenString, &claims,
		func(token *jwt.Token) (interface{}, error) {
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	return claims.UserID, nil
}
