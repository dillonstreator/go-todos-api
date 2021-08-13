package domain

import (
	"time"

	"github.com/DillonStreator/todos/entityid"
)

type User struct {
	ID         entityid.ID `json:"id"`
	CreatedAt  time.Time   `json:"createdAt"`
	LastSeenAt time.Time   `json:"lastSeenAt"`
	Todos      Todos       `json:"todos"`
}

type Todos []*Todo

func (todos Todos) FindByID(id entityid.ID) *Todo {
	if index := todos.FindIndexByID(id); index != -1 {
		return todos[index]
	}
	return &Todo{}
}

func (todos Todos) FindIndexByID(id entityid.ID) int {
	for i, todo := range todos {
		if todo.ID == id {
			return i
		}
	}
	return -1
}

type Todo struct {
	ID          entityid.ID `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Completed   bool        `json:"completed"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}
