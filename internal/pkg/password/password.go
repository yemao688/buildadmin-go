package password

import "golang.org/x/crypto/bcrypt"

func Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func Compare(encoded, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(plain))
}
