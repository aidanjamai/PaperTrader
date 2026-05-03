package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secretKey []byte
}

func NewJWTService(secretKey string) *JWTService {
	return &JWTService{secretKey: []byte(secretKey)}
}

func (j *JWTService) GenerateToken(userID, email string) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secretKey)
}

func (j *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	// Pin the signing method explicitly. Without WithValidMethods, a future
	// refactor that adds (say) an RSA key handler would risk algorithm-
	// confusion attacks. Locks the parser to HS256 — the only thing
	// GenerateToken issues — so any other alg (including "none") is rejected.
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(token *jwt.Token) (interface{}, error) {
			return j.secretKey, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
	)

	if err != nil || !token.Valid {
		return nil, err
	}

	return claims, nil
}
