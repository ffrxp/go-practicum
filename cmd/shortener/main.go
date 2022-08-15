package main

import (
	"github.com/ffrxp/go-practicum/internal/app"
	"log"
	"net/http"
)

func main() {
	config := app.InitConfig()
	if config.DatabasePath != "" {
		storage, err := app.NewDatabaseStorage(config.DatabasePath)
		if err == nil {
			defer storage.Close()
			sa := app.ShortenerApp{Storage: storage,
				BaseAddress:  config.BaseAddress,
				DatabasePath: config.DatabasePath}
			log.Fatal(http.ListenAndServe(config.ServerAddress, app.NewShortenerHandler(&sa)))
		}
		log.Printf("Can't connect to database or init tables. Error:%s", err.Error())
		dataStorage := app.NewDataStorage(config.StoragePath)
		defer dataStorage.Close()
		sa := app.ShortenerApp{Storage: dataStorage,
			BaseAddress:  config.BaseAddress,
			DatabasePath: config.DatabasePath}
		log.Fatal(http.ListenAndServe(config.ServerAddress, app.NewShortenerHandler(&sa)))
	}
	dataStorage := app.NewDataStorage(config.StoragePath)
	defer dataStorage.Close()
	sa := app.ShortenerApp{Storage: dataStorage,
		BaseAddress:  config.BaseAddress,
		DatabasePath: config.DatabasePath}
	log.Fatal(http.ListenAndServe(config.ServerAddress, app.NewShortenerHandler(&sa)))
}
