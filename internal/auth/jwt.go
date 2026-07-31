package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const accessTokenExpiry = 15 * time.Minute

type AccessTokenClaims struct {
	UserId string `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(userId, secret string) (string, error) {
	claims := AccessTokenClaims{
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signed, nil
}

func ValidateAccessToken(tokenString, secret string) (*AccessTokenClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&AccessTokenClaims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secret), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*AccessTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
