package auth

import "golang.org/x/crypto/bcrypt"

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
