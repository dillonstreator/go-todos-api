package entityid

import "github.com/google/uuid"

type ID string

func (i ID) String() string {
	return string(i)
}

type idGenerator struct {
}

func newIDGenerator() *idGenerator {
	return &idGenerator{}
}

func (idg idGenerator) Generate() ID {
	return ID(uuid.New().String())
}

var Generator = newIDGenerator()
