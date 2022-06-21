package app

import "os"

const DefaultServerAddress = ":8080"
const DefaultBaseAddress = "http://localhost:8080"

func GetServerAddress() string {
	val, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok || val == "" {
		return DefaultServerAddress
	}
	return val
}

func GetBaseAddress() string {
	val, ok := os.LookupEnv("BASE_URL")
	if !ok || val == "" {
		return DefaultBaseAddress
	}
	return val
}

func GetStoragePath() string {
	path, ok := os.LookupEnv("FILE_STORAGE_PATH")
	if !ok {
		return ""
	}
	return path
}
