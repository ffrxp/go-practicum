package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"os"
)
import "crypto/rand"

const DefaultServerAddress = ":8080"
const DefaultBaseAddress = "http://localhost:8080"
const CookieName = "URLs"

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

func GenerateRandom(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func SignMsg(msg []byte, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	dst := h.Sum(nil)
	return dst
}

func GetUserToken(UID string) string {
	// Make token. It is learning realization, so token will be not enough secured for real implementation
	return fmt.Sprintf("token%s", string(UID))
}

func CheckSign(msg []byte, sign []byte, key []byte) bool {
	checkingSign := SignMsg(msg, key)
	if hmac.Equal(sign, checkingSign) {
		return true
	} else {
		return false
	}
}
