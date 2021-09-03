package passwords

import "golang.org/x/crypto/bcrypt"

func Hash(unhashedPassword []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(unhashedPassword, bcrypt.DefaultCost)
}

func Compare(hashedPassword, unhashedPassword []byte) error {
	return bcrypt.CompareHashAndPassword(hashedPassword, unhashedPassword)
}
