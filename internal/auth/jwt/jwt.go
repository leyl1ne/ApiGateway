package jwt

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type Payload struct {
	UserID    string `json:"user_id"`
	UserRole  string `json:"user_role"`
	CompanyID string `json:"company_id"`
}

type Claims struct {
	Payload
	jwt.RegisteredClaims
}

type JWTValidator struct {
	jwtSecret []byte
}

// NewJWTValidator создаёт валидатор JWT.
// В отличие от генератора из UserService, gateway'ю нужна только
// функция Validate — генерировать токены он не будет.
func NewJWTValidator(cfg Config) *JWTValidator {
	return &JWTValidator{
		jwtSecret: []byte(cfg.Secret),
	}
}

// Validate проверяет подпись и срок действия JWT-токена,
// возвращает полезную нагрузку (Payload) с UserID, UserRole, CompanyID.
func (v *JWTValidator) Validate(tokenString string) (Payload, error) {
	const op = "auth.jwt.Validate"

	claims := &Claims{}
	var payload Payload

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%s: %w", op, ErrInvalidToken)
		}
		return v.jwtSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return payload, fmt.Errorf("%s: %w", op, ErrExpiredToken)
		}
		return payload, fmt.Errorf("%s: parse token: %w", op, err)
	}

	if !token.Valid {
		return payload, fmt.Errorf("%s: %w", op, ErrInvalidToken)
	}

	return Payload{
		UserID:    claims.UserID,
		UserRole:  claims.UserRole,
		CompanyID: claims.CompanyID,
	}, nil
}
