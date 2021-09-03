package jwt

import (
	"errors"
	"os"
	"time"

	"github.com/DillonStreator/todos/entityid"
	jwtgo "github.com/dgrijalva/jwt-go"
)

type Input struct {
	UserID entityid.ID
	Email  string
}
type claim struct {
	UserID entityid.ID `json:"userId"`
	Email  string      `json:"email"`
	jwtgo.StandardClaims
}

func getJWTSecret() string {
	return os.Getenv("JWT_SECRET")
}

func SignJWT(input Input) (string, error) {
	c := claim{
		UserID: input.UserID,
		Email:  input.Email,
		StandardClaims: jwtgo.StandardClaims{
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		},
	}
	token := jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, c)
	return token.SignedString([]byte(getJWTSecret()))
}

func Verify(jwt string) (claim, error) {
	token, err := jwtgo.ParseWithClaims(
		jwt,
		&claim{},
		func(token *jwtgo.Token) (interface{}, error) {
			return []byte(getJWTSecret()), nil
		},
	)
	if err != nil {
		return claim{}, err
	}

	claims, ok := token.Claims.(*claim)
	if !ok {
		return claim{}, errors.New("failed to parse claim")
	}

	if claims.ExpiresAt < time.Now().UTC().Unix() {
		return claim{}, errors.New("jwt is expired")
	}

	return *claims, nil
}
