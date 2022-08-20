package common

import (
	"crypto/hmac"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"strconv"
)

const defaultServerAddress = ":8080"
const defaultBaseAddress = "http://localhost:8080"

type Config struct {
	ServerAddress string
	BaseAddress   string
	StoragePath   string
	DatabasePath  string
}

func InitConfig() *Config {
	var conf Config

	defServerAddress, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok || defServerAddress == "" {
		defServerAddress = defaultServerAddress
	}
	defBaseAddress, ok := os.LookupEnv("BASE_URL")
	if !ok || defBaseAddress == "" {
		defBaseAddress = defaultBaseAddress
	}
	defStoragePath, ok := os.LookupEnv("FILE_STORAGE_PATH")
	if !ok {
		defStoragePath = ""
	}
	defDatabasePath, ok := os.LookupEnv("DATABASE_DSN")
	if !ok {
		defDatabasePath = ""
	}

	flag.StringVar(&(conf.ServerAddress), "a", defServerAddress, "Start server address.")
	flag.StringVar(&(conf.BaseAddress), "b", defBaseAddress, "Base address for short URLs")
	flag.StringVar(&(conf.StoragePath), "f", defStoragePath, "Path for storage of short URLs")
	flag.StringVar(&(conf.DatabasePath), "d", defDatabasePath, "Path for connect to database")
	flag.Parse()

	return &conf
}

func SignMsg(msg []byte, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	dst := h.Sum(nil)
	return dst
}

func GetUserToken(UID int) string {
	// Make token. It is learning realization, so token will be not enough secured for real implementation
	strUID := strconv.Itoa(UID)
	return fmt.Sprintf("token%s", strUID)
}
