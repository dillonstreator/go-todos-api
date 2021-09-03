package storage

import (
	"reflect"

	"github.com/DillonStreator/todos/domain"
	"github.com/eleanorhealth/milo"
)

// MiloEntityModelMap is used by Milo to map domain entities to storage models.
var MiloEntityModelMap = milo.EntityModelMap{
	reflect.TypeOf(&domain.User{}): milo.ModelConfig{
		Model: reflect.TypeOf(&user{}),
		FieldColumnMap: milo.FieldColumnMap{
			"Email": "email",
		},
	},
}
