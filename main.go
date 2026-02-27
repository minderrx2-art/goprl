package main

import (
	"goprl/internal/app"
	"goprl/internal/config"
)

func main() {
	config, err := config.NewConfig()
	if err != nil {
		panic("CONFIG creation failed: " + err.Error())
	}
	app, err := app.NewApp(config)
	if err != nil {
		panic("APP creation failed: " + err.Error())
	}
	defer app.Close()
	if err := app.Run(); err != nil {
		panic("APP run failed: " + err.Error())
	}
}
