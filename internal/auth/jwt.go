package auth

import (
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

const accessTokenExpiry = 15 * time.Minute

type AccessTokenClaims struct {
	Purpose    string       `json:"purpose"` // 'access' ONLY
	UserId     string       `json:"user_id"`
	BusinessId string       `json:"business_id"` // empty for identity-only sessions
	LocationId string       `json:"location_id"` // empty for admin-only sessions
	IsAdmin    bool         `json:"is_admin"`
	Role       domain.LRole `json:"role"` // empty unless a location role applies
	jwt.RegisteredClaims
}

func GenerateAccessToken(
	userId,
	businessId,
	locationId string,
	isAdmin bool,
	role domain.LRole,
	secret string,
) (string, error) {
	claims := AccessTokenClaims{
		Purpose:    "access",
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
	if !ok || !token.Valid || claims.Purpose != "access" {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
