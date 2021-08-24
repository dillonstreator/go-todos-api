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
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

type userContextKey string

var USER_CONTEXT_KEY = userContextKey("user")

func requestGetUser(r *http.Request) *domain.User {
	return r.Context().Value(USER_CONTEXT_KEY).(*domain.User)
}
func requestSetUser(r *http.Request, user *domain.User) *http.Request {
	ctx := context.WithValue(r.Context(), USER_CONTEXT_KEY, user)
	return r.WithContext(ctx)
}

func newInMemoryLimiterMiddleware(r limiter.Rate) *stdlib.Middleware {
	store := memory.NewStore()
	limiter := limiter.New(store, r)
	return stdlib.NewMiddleware(limiter)
}

type ErrorResponseError struct {
	Message string `json:"message"`
	Field   string `json:"field"`
}
type ErrorResponse struct {
	Errors []ErrorResponseError `json:"errors"`
}

func respondError(rw http.ResponseWriter, status int, errors ErrorResponse) {
	bytes, err := json.Marshal(errors)
	if err != nil {
		respondError(rw, http.StatusInternalServerError, ErrorResponse{
			Errors: []ErrorResponseError{{Message: "Something went wrong"}},
		})
		return
	}
	rw.WriteHeader(status)
	rw.Write(bytes)
}

func getMux() http.Handler {
	r := chi.NewRouter()

	requestLimiter := newInMemoryLimiterMiddleware(limiter.Rate{
		Period: 1 * time.Second,
		Limit:  5,
	})
	r.Use(requestLimiter.Handler)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("Access-Control-Allow-Origin", "*")
			rw.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
			rw.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			if r.Method == "OPTIONS" {
				return
			}
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
		userCreationLimiter := newInMemoryLimiterMiddleware(limiter.Rate{
			Period: 1 * time.Hour,
			Limit:  5,
		})
		usersRouter.With(userCreationLimiter.Handler).Post("/", func(rw http.ResponseWriter, r *http.Request) {
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
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
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
					respondError(rw, http.StatusUnauthorized, ErrorResponse{
						Errors: []ErrorResponseError{{Message: "Not authorized"}},
					})
					return
				}
				var user = &domain.User{}
				store.FindByID(user, userID)
				if user.ID == "" {
					respondError(rw, http.StatusNotFound, ErrorResponse{
						Errors: []ErrorResponseError{{Message: "User not found"}},
					})
					return
				}

				user.LastSeenAt = time.Now()
				err := store.Save(context.Background(), user)
				if err != nil {
					respondError(rw, http.StatusInternalServerError, ErrorResponse{
						Errors: []ErrorResponseError{{Message: err.Error()}},
					})
					return
				}

				next.ServeHTTP(rw, requestSetUser(r, user))
			})
		})

		todosRouter.Get("/", func(rw http.ResponseWriter, r *http.Request) {
			user := requestGetUser(r)
			var todos = make([]*domain.Todo, 0)
			todos = append(todos, user.Todos...)
			bytes, err := json.Marshal(todos)
			if err != nil {
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			rw.WriteHeader(http.StatusOK)
			rw.Write(bytes)
		})
		todoCreationLimiter := newInMemoryLimiterMiddleware(limiter.Rate{
			Period: 1 * time.Hour,
			Limit:  100,
		})
		todosRouter.With(todoCreationLimiter.Handler).Post("/", func(rw http.ResponseWriter, r *http.Request) {
			user := requestGetUser(r)

			var todo = &domain.Todo{}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			err := decoder.Decode(todo)
			if err != nil {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			todo.ID = entityid.Generator.Generate()
			todo.CreatedAt = time.Now()
			todo.UpdatedAt = time.Now()
			user.Todos = append(user.Todos, todo)
			err = store.Save(context.Background(), user)
			if err != nil {
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			bytes, err := json.Marshal(todo)
			if err != nil {
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			rw.WriteHeader(http.StatusCreated)
			rw.Write(bytes)
		})
		todosRouter.Put("/{todoID}", func(rw http.ResponseWriter, r *http.Request) {
			user := requestGetUser(r)

			todoID := entityid.ID(chi.URLParam(r, "todoID"))
			todo := user.Todos.FindByID(todoID)
			if todo.ID == "" {
				respondError(rw, http.StatusNotFound, ErrorResponse{
					Errors: []ErrorResponseError{{Message: "Todo not found"}},
				})
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
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			todo.Completed = updatedTodo.Completed
			todo.Title = updatedTodo.Title
			todo.Description = updatedTodo.Description
			todo.UpdatedAt = time.Now()

			err = store.Save(context.Background(), user)
			if err != nil {
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			bytes, err := json.Marshal(todo)
			if err != nil {
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			rw.WriteHeader(http.StatusOK)
			rw.Write(bytes)
		})
		todosRouter.Delete("/{todoID}", func(rw http.ResponseWriter, r *http.Request) {
			user := requestGetUser(r)

			todoID := entityid.ID(chi.URLParam(r, "todoID"))
			index := user.Todos.FindIndexByID(todoID)
			if index == -1 {
				respondError(rw, http.StatusNotFound, ErrorResponse{
					Errors: []ErrorResponseError{{Message: "Todo not found"}},
				})
				return
			}

			user.Todos = append(user.Todos[:index], user.Todos[index+1:]...)
			err := store.Save(context.Background(), user)
			if err != nil {
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
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
	HOST := getEnv("HOST", "")
	addr := fmt.Sprintf("%s:%s", HOST, PORT)
	fmt.Printf("Starting server at %s\n", addr)
	return http.ListenAndServe(addr, mux)
}
