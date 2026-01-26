package main

import (
	"fmt"
	"goprl/internal/api"
	"goprl/internal/app"
	"goprl/internal/config"
	"net/http"
)

func main() {
	config, err := config.NewConfig()
	if err != nil {
		panic("CONFIG: " + err.Error())
	}
	app, err := app.NewApp(config)
	if err != nil {
		panic("APP: " + err.Error())
	}
	defer app.Close()

	mux := http.NewServeMux()
	app.Handler.RegisterRoutes(mux)

	fmt.Println("URL Shortener starting on http://localhost:" + app.Config.Port)
	if err := http.ListenAndServe(":"+app.Config.Port,
		api.RequestIDMiddleware(
			api.LoggingMiddleware(app.Logger)(mux),
		)); err != nil {
		panic("SERVER: " + err.Error())
	}
}
