package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	pword, err := bcrypt.GenerateFromPassword([]byte(password), 1)
	if err != nil {
		return "", err
	}
	return string(pword), nil
}

func CheckPasswordHash(password, hash string) error {
	result := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return result
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Issuer: "chirpy", IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)), Subject: userID.String()})
	tokenString, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	}

	token, err := jwt.ParseWithClaims(tokenString, jwt.RegisteredClaims{}, keyFunc)
	if err != nil {
		return uuid.Nil, err
	}
	userId, claimErr := token.Claims.GetSubject()
	if claimErr != nil {
		return uuid.Nil, claimErr
	}
	uuidParse, uuidErr := uuid.Parse(userId)
	if uuidErr != nil {
		return uuid.Nil, uuidErr
	}
	return uuidParse, nil
}
