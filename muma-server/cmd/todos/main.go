package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"muma/internal/api"
	"muma/internal/db"
)

func main() {
	r := chi.NewRouter()

	db := db.New().Connect()

	r.Use(middleware.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Muma-Stream"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	r.Group(func(r chi.Router) {
		todosApi := api.NewTodosApi(db)

		r.Get("/todos/{sessionID}", todosApi.GetTodos)
		// TODO: Make this so that the task is the request body
		r.Post("/todos/{sessionID}/{task}", todosApi.CreateTodo)
		r.Put("/api/v1/todos/{sessionID}/{taskID}", todosApi.UpdateTodo)
	})

	log.Println("starting todos server at https://localhost:3000")

	err := http.ListenAndServeTLS("localhost:3000", "cert.pem", "key.pem", r)

	if err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
