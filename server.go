package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/DillonStreator/todos/domain"
	"github.com/DillonStreator/todos/entityid"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func getMux() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Add("Content-Type", "application/json")
			next.ServeHTTP(rw, r)
		})
	})

	r.Get("/", func(rw http.ResponseWriter, r *http.Request) {
		http.Redirect(rw, r, "/status", http.StatusPermanentRedirect)
	})
	r.Get("/status", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("🌈"))
	})

	r.Route("/users", func(usersRouter chi.Router) {
		usersRouter.Post("/", func(rw http.ResponseWriter, r *http.Request) {
			var user = &domain.User{
				ID:         entityid.Generator.Generate(),
				CreatedAt:  time.Now(),
				LastSeenAt: time.Now(),
				Todos:      make([]*domain.Todo, 0),
			}
			store.Save(context.Background(), user)
			bytes, err := json.Marshal(user)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			rw.WriteHeader(http.StatusCreated)
			rw.Write(bytes)
		})
	})

	r.Route("/todos", func(todosRouter chi.Router) {
		todosRouter.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				userID := r.Header.Get("Authorization")
				if userID == "" {
					rw.WriteHeader(http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(rw, r)
			})
		})

		todosRouter.Get("/", func(rw http.ResponseWriter, r *http.Request) {
			var user = &domain.User{}
			store.FindByID(user, r.Header.Get("Authorization"))
			if user.ID == "" {
				rw.WriteHeader(http.StatusNotFound)
				return
			}
			var todos = make([]*domain.Todo, 0)
			todos = append(todos, user.Todos...)
			bytes, err := json.Marshal(todos)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			rw.WriteHeader(http.StatusOK)
			rw.Write(bytes)
		})
		todosRouter.Post("/", func(rw http.ResponseWriter, r *http.Request) {
			var user = &domain.User{}
			store.FindByID(user, r.Header.Get("Authorization"))
			if user.ID == "" {
				rw.WriteHeader(http.StatusNotFound)
				return
			}

			var todo = &domain.Todo{}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			err := decoder.Decode(todo)
			if err != nil {
				fmt.Println(err)
				rw.WriteHeader(http.StatusBadRequest)
				rw.Write([]byte(err.Error()))
				return
			}

			todo.ID = entityid.Generator.Generate()
			todo.CreatedAt = time.Now()
			todo.UpdatedAt = time.Now()
			user.Todos = append(user.Todos, todo)
			err = store.Save(context.Background(), user)
			if err != nil {
				fmt.Println(err)
				rw.WriteHeader(http.StatusInternalServerError)
				rw.Write([]byte(err.Error()))
				return
			}

			bytes, err := json.Marshal(todo)
			if err != nil {
				fmt.Println(err)
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			rw.WriteHeader(http.StatusCreated)
			rw.Write(bytes)
		})
		todosRouter.Put("/{todoID}", func(rw http.ResponseWriter, r *http.Request) {
			var user = &domain.User{}
			store.FindByID(user, r.Header.Get("Authorization"))
			if user.ID == "" {
				rw.WriteHeader(http.StatusNotFound)
				return
			}

			todoID := entityid.ID(chi.URLParam(r, "todoID"))
			todo := user.Todos.FindByID(todoID)
			if todo.ID == "" {
				rw.WriteHeader(http.StatusNotFound)
				return
			}

			var updatedTodo = &domain.Todo{
				Completed:   todo.Completed,
				Title:       todo.Title,
				Description: todo.Description,
			}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			err := decoder.Decode(updatedTodo)
			if err != nil {
				fmt.Println(err)
				rw.WriteHeader(http.StatusBadRequest)
				rw.Write([]byte(err.Error()))
				return
			}

			todo.Completed = updatedTodo.Completed
			todo.Title = updatedTodo.Title
			todo.Description = updatedTodo.Description
			todo.UpdatedAt = time.Now()

			err = store.Save(context.Background(), user)
			if err != nil {
				fmt.Println(err)
				rw.WriteHeader(http.StatusInternalServerError)
				rw.Write([]byte(err.Error()))
				return
			}

			bytes, err := json.Marshal(todo)
			if err != nil {
				fmt.Println(err)
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			rw.WriteHeader(http.StatusOK)
			rw.Write(bytes)
		})
		todosRouter.Delete("/{todoID}", func(rw http.ResponseWriter, r *http.Request) {
			var user = &domain.User{}
			store.FindByID(user, r.Header.Get("Authorization"))
			if user.ID == "" {
				rw.WriteHeader(http.StatusNotFound)
				return
			}

			todoID := entityid.ID(chi.URLParam(r, "todoID"))
			index := user.Todos.FindIndexByID(todoID)
			if index == -1 {
				rw.WriteHeader(http.StatusNotFound)
				return
			}

			user.Todos = append(user.Todos[:index], user.Todos[index+1:]...)
			err := store.Save(context.Background(), user)
			if err != nil {
				fmt.Println(err)
				rw.WriteHeader(http.StatusInternalServerError)
				rw.Write([]byte(err.Error()))
				return
			}

			rw.WriteHeader(http.StatusNoContent)
		})
	})

	return r
}

func startServer() error {
	mux := getMux()
	PORT := getEnv("PORT", "4000")
	return http.ListenAndServe(fmt.Sprintf(":%s", PORT), mux)
}
