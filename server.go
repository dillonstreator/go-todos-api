package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DillonStreator/todos/domain"
	"github.com/DillonStreator/todos/entityid"
	"github.com/DillonStreator/todos/jwt"
	"github.com/DillonStreator/todos/passwords"
	"github.com/eleanorhealth/milo"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

var durationUnitsMap = map[string]time.Duration{
	"ms":  time.Millisecond,
	"s":   time.Second,
	"min": time.Minute,
	"hr":  time.Hour,
}

type limiterDefaultOpts struct {
	Units        time.Duration
	UnitQuantity int64
	Limit        int64
}

func onlySomeEnvsSet(envKeys ...string) bool {
	if len(envKeys) <= 1 {
		return false
	}

	_, ok := os.LookupEnv(envKeys[0])
	isset := ok
	for _, key := range envKeys[1:] {
		_, ok := os.LookupEnv(key)
		if ok != isset {
			return true
		}
	}
	return false
}

func noEnvsSet(envKeys ...string) bool {
	for _, key := range envKeys {
		_, ok := os.LookupEnv(key)
		if ok {
			return false
		}
	}
	return true
}

func getRequestLimiterRateEnv(key string, defaultOpts limiterDefaultOpts) limiter.Rate {
	unitsKey := fmt.Sprintf("%s_REQUEST_LIMITER_UNITS", key)
	quantityKey := fmt.Sprintf("%s_REQUEST_LIMITER_QUANTITY", key)
	limitKey := fmt.Sprintf("%s_REQUEST_LIMITER_LIMIT", key)
	envKeys := []string{unitsKey, quantityKey, limitKey}
	if onlySomeEnvsSet(envKeys...) {
		log.Fatalf("must either specify all or none of envs: %s", strings.Join(envKeys, ","))
	} else if noEnvsSet(envKeys...) {
		return limiter.Rate{
			Period: defaultOpts.Units * time.Duration(defaultOpts.UnitQuantity),
			Limit:  defaultOpts.Limit,
		}
	}

	units := getEnv(unitsKey, "")
	durationUnits, ok := durationUnitsMap[units]
	if !ok {
		log.Fatalf("invalid units %s for key %s", units, unitsKey)
	}
	quantity := getEnv(quantityKey, "")
	quantityInt, err := strconv.ParseInt(quantity, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	limit := getEnv(limitKey, "")
	limitInt, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	return limiter.Rate{
		Period: durationUnits * time.Duration(quantityInt),
		Limit:  limitInt,
	}
}

type userContextKey string

var USER_CONTEXT_KEY = userContextKey("user")

func requestGetUser(r *http.Request) *domain.User {
	return r.Context().Value(USER_CONTEXT_KEY).(*domain.User)
}
func requestSetUser(r *http.Request, user *domain.User) *http.Request {
	ctx := context.WithValue(r.Context(), USER_CONTEXT_KEY, user)
	return r.WithContext(ctx)
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

func tooManyRequestsHandler(rw http.ResponseWriter, r *http.Request) {
	respondError(rw, http.StatusTooManyRequests, ErrorResponse{
		Errors: []ErrorResponseError{{Message: "Too many requests... Slow down"}},
	})
}

func newInMemoryLimiterMiddleware(r limiter.Rate) *stdlib.Middleware {
	store := memory.NewStore()
	limiter := limiter.New(store, r)
	limiterMiddleware := stdlib.NewMiddleware(limiter)
	limiterMiddleware.OnLimitReached = tooManyRequestsHandler
	return limiterMiddleware
}

type userCredentialsInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func getMux() http.Handler {
	r := chi.NewRouter()

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

	requestLimiter := newInMemoryLimiterMiddleware(
		getRequestLimiterRateEnv("GLOBAL", limiterDefaultOpts{
			Units:        time.Second,
			UnitQuantity: 1,
			Limit:        2,
		}),
	)
	r.Use(requestLimiter.Handler)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)

	r.Get("/", func(rw http.ResponseWriter, r *http.Request) {
		http.Redirect(rw, r, "/status", http.StatusPermanentRedirect)
	})
	r.Get("/status", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("🌈"))
	})

	r.Route("/sessions", func(sessionsRouter chi.Router) {
		sessionCreateLimiter := newInMemoryLimiterMiddleware(
			getRequestLimiterRateEnv("SIGN_IN", limiterDefaultOpts{
				Units:        time.Minute,
				UnitQuantity: 5,
				Limit:        20,
			}),
		)
		sessionsRouter.With(sessionCreateLimiter.Handler).Post("/", func(rw http.ResponseWriter, r *http.Request) {
			var userCredsInput = userCredentialsInput{}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			err := decoder.Decode(&userCredsInput)
			if err != nil {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: "invalid input"}},
				})
				return
			}

			user := &domain.User{}
			err = store.FindOneBy(user, milo.Equal("Email", userCredsInput.Email))
			if err != nil && err != milo.ErrNotFound {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}
			if user.ID == "" {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: "incorrect credentials"}},
				})
				return
			}
			if err = passwords.Compare([]byte(user.Password), []byte(userCredsInput.Password)); err != nil {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: "incorrect credentials"}},
				})
				return
			}

			user.LastSeenAt = time.Now()
			store.Save(context.Background(), user)
			jwtInput := jwt.Input{
				UserID: user.ID,
				Email:  user.Email,
			}
			token, err := jwt.SignJWT(jwtInput)
			if err != nil {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			bytes, err := json.Marshal(struct {
				Token string `json:"token"`
			}{Token: token})
			if err != nil {
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}

			rw.WriteHeader(http.StatusOK)
			rw.Write(bytes)
		})
	})

	r.Route("/users", func(usersRouter chi.Router) {
		userCreationLimiter := newInMemoryLimiterMiddleware(
			getRequestLimiterRateEnv("USER_CREATION", limiterDefaultOpts{
				Units:        time.Hour,
				UnitQuantity: 1,
				Limit:        5,
			}),
		)
		usersRouter.With(userCreationLimiter.Handler).Post("/", func(rw http.ResponseWriter, r *http.Request) {
			var userCredsInput = userCredentialsInput{}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			err := decoder.Decode(&userCredsInput)
			if err != nil {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: "invalid input"}},
				})
				return
			}
			if _, err := mail.ParseAddress(userCredsInput.Email); err != nil || len(userCredsInput.Password) < 8 {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: "must provide a valid email address and a password with atleast 8 characters"}},
				})
				return
			}

			existingUser := &domain.User{}
			err = store.FindOneBy(existingUser, milo.Equal("Email", userCredsInput.Email))
			if err != nil && err != milo.ErrNotFound {
				respondError(rw, http.StatusBadRequest, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}
			if existingUser.ID != "" {
				respondError(rw, http.StatusConflict, ErrorResponse{
					Errors: []ErrorResponseError{{Message: "email already in use"}},
				})
				return
			}

			hashedPassword, err := passwords.Hash([]byte(userCredsInput.Password))
			if err != nil {
				respondError(rw, http.StatusInternalServerError, ErrorResponse{
					Errors: []ErrorResponseError{{Message: err.Error()}},
				})
				return
			}
			var user = &domain.User{
				ID:         entityid.Generator.Generate(),
				CreatedAt:  time.Now(),
				LastSeenAt: time.Now(),
				Todos:      make([]*domain.Todo, 0),
				Email:      userCredsInput.Email,
				Password:   string(hashedPassword),
			}
			store.Save(context.Background(), user)
			bytes, err := json.Marshal(user)
			if err != nil {
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
				token := r.Header.Get("Authorization")
				if token == "" {
					respondError(rw, http.StatusUnauthorized, ErrorResponse{
						Errors: []ErrorResponseError{{Message: "Not authorized"}},
					})
					return
				}
				claim, err := jwt.Verify(token)
				if err != nil {
					respondError(rw, http.StatusUnauthorized, ErrorResponse{
						Errors: []ErrorResponseError{{Message: err.Error()}},
					})
					return
				}

				var user = &domain.User{}
				store.FindByID(user, claim.UserID)
				if user.ID == "" {
					respondError(rw, http.StatusNotFound, ErrorResponse{
						Errors: []ErrorResponseError{{Message: "User not found"}},
					})
					return
				}

				user.LastSeenAt = time.Now()
				err = store.Save(context.Background(), user)
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
		todoCreationLimiter := newInMemoryLimiterMiddleware(
			getRequestLimiterRateEnv("TODO_CREATION", limiterDefaultOpts{
				Units:        time.Hour,
				UnitQuantity: 1,
				Limit:        100,
			}),
		)
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
