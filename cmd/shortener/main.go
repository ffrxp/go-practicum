package main

import (
	"flag"
	"fmt"
	"github.com/ffrxp/go-practicum/internal/app"
	"log"
	"net/http"
)

func main() {
	serverAddress := flag.String("a", app.GetServerAddress(), "Start server address.")
	baseAddress := flag.String("b", app.GetBaseAddress(), "Base address for short URLs")
	storagePath := flag.String("f", app.GetStoragePath(), "Path for storage of short URLs")
	databasePath := flag.String("d", app.GetDatabasePath(), "Path for connect to database")
	flag.Parse()

	if *databasePath != "" {
		storage, err := app.NewDatabaseStorage(*databasePath)
		defer storage.Close()
		if err == nil {
			sa := app.ShortenerApp{Storage: storage,
				BaseAddress:  *baseAddress,
				DatabasePath: *databasePath}
			log.Fatal(http.ListenAndServe(*serverAddress, app.NewShortenerHandler(&sa)))
		}
		log.Println(fmt.Sprintf("Can't connect to database or init tables. Error:%s", err.Error()))
		dataStorage := app.NewDataStorage(*storagePath)
		defer dataStorage.Close()
		sa := app.ShortenerApp{Storage: dataStorage,
			BaseAddress:  *baseAddress,
			DatabasePath: *databasePath}
		log.Fatal(http.ListenAndServe(*serverAddress, app.NewShortenerHandler(&sa)))
	}
	dataStorage := app.NewDataStorage(*storagePath)
	defer dataStorage.Close()
	sa := app.ShortenerApp{Storage: dataStorage,
		BaseAddress:  *baseAddress,
		DatabasePath: *databasePath}
	log.Fatal(http.ListenAndServe(*serverAddress, app.NewShortenerHandler(&sa)))
}
