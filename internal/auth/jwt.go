package auth

import (
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const accessTokenExpiry = 15 * time.Minute

type Claims struct {
	UserId     string       `json:"user_id"`
	BusinessId string       `json:"business_id"`
	LocationId string       `json:"location_id"` // empty for admin-only sessions
	IsAdmin    bool         `json:"is_admin"`
	Role       domain.LRole `json:"role"` // empty for admin-only sessions
	jwt.RegisteredClaims
}

func GenerateJWT(
	userId,
	businessId,
	locationId string,
	isAdmin bool,
	role domain.LRole,
	secret string,
) (string, error) {
	claims := Claims{
		UserId:     userId,
		BusinessId: businessId,
		IsAdmin:    isAdmin,
		LocationId: locationId,
		Role:       role,
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

func ValidateJWT(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
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

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
