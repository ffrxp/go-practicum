package main

import (
	"flag"
	"github.com/ffrxp/go-practicum/internal/app"
	"log"
	"net/http"
)

func main() {
	serverAddress := flag.String("a", app.GetServerAddress(), "Start server address.")
	baseAddress := flag.String("b", app.GetBaseAddress(), "Base address for short URLs")
	storagePath := flag.String("f", app.GetStoragePath(), "Path for storage of short URLs")
	flag.Parse()

	storage := app.NewDataStorage(*storagePath)
	defer storage.Close()
	sa := app.ShortenerApp{Storage: storage, BaseAddress: *baseAddress}
	log.Fatal(http.ListenAndServe(*serverAddress, app.NewShortenerHandler(&sa)))
}