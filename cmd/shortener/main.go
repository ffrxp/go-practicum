package main

import (
	"github.com/ffrxp/go-practicum/internal/app"
	"github.com/ffrxp/go-practicum/internal/common"
	"github.com/ffrxp/go-practicum/internal/handlers"
	"github.com/ffrxp/go-practicum/internal/storage"
	"log"
	"net/http"
)

func main() {
	config := common.InitConfig()
	if config.DatabasePath != "" {
		appStorage, err := storage.NewDatabaseStorage(config.DatabasePath)
		if err == nil {
			defer appStorage.Close()
			sa := app.ShortenerApp{Storage: appStorage,
				BaseAddress:  config.BaseAddress,
				DatabasePath: config.DatabasePath}
			log.Fatal(http.ListenAndServe(config.ServerAddress, handlers.NewShortenerHandler(&sa)))
		}
		log.Printf("Can't connect to database or init tables. Error:%s", err.Error())
		dataStorage := storage.NewDataStorage(config.StoragePath)
		defer dataStorage.Close()
		sa := app.ShortenerApp{Storage: dataStorage,
			BaseAddress:  config.BaseAddress,
			DatabasePath: config.DatabasePath}
		log.Fatal(http.ListenAndServe(config.ServerAddress, handlers.NewShortenerHandler(&sa)))
	}
	dataStorage := storage.NewDataStorage(config.StoragePath)
	defer dataStorage.Close()
	sa := app.ShortenerApp{Storage: dataStorage,
		BaseAddress:  config.BaseAddress,
		DatabasePath: config.DatabasePath}
	log.Fatal(http.ListenAndServe(config.ServerAddress, handlers.NewShortenerHandler(&sa)))
}
