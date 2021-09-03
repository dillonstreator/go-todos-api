package storage

import (
	"time"

	"github.com/DillonStreator/todos/domain"
	"github.com/DillonStreator/todos/entityid"
	"github.com/eleanorhealth/milo"
)

type user struct {
	ID         string    `pg:"id"`
	Email      string    `pg:"email"`
	Password   string    `pg:"password"`
	CreatedAt  time.Time `pg:"created_at"`
	LastSeenAt time.Time `pg:"last_seen_at"`
	Todos      []*todo   `pg:"rel:has-many"`
}

var _ milo.Model = (*user)(nil)

type todo struct {
	ID          string    `pg:"id"`
	UserID      string    `pg:"user_id"`
	Title       string    `pg:"title"`
	Description string    `pg:"description"`
	Completed   bool      `pg:"completed"`
	CreatedAt   time.Time `pg:"created_at"`
	UpdatedAt   time.Time `pg:"updated_at"`
}

func (u *user) FromEntity(e interface{}) error {
	entity := e.(*domain.User)

	u.ID = entity.ID.String()
	u.Email = entity.Email
	u.Password = entity.Password

	u.CreatedAt = entity.CreatedAt
	u.LastSeenAt = entity.LastSeenAt

	for _, t := range entity.Todos {
		u.Todos = append(u.Todos, &todo{
			ID:          t.ID.String(),
			UserID:      u.ID,
			Title:       t.Title,
			Description: t.Description,
			Completed:   t.Completed,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}

	return nil
}

func (u *user) ToEntity() (interface{}, error) {
	entity := &domain.User{}

	entity.ID = entityid.ID(u.ID)
	entity.Email = u.Email
	entity.Password = u.Password

	entity.CreatedAt = u.CreatedAt
	entity.LastSeenAt = u.LastSeenAt

	for _, t := range u.Todos {
		entity.Todos = append(entity.Todos, &domain.Todo{
			ID:          entityid.ID(t.ID),
			Title:       t.Title,
			Description: t.Description,
			Completed:   t.Completed,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}

	return entity, nil
}
