package main

import (
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/scrypt"
	"strconv"
	"strings"
)

var (
	errHashMissmatch = errors.New("Hash and data does not match")
	errInvalidKey    = errors.New("Invalid key")
)

type scryptKey struct {
	N      int
	p      int
	r      int
	keyLen int
	salt   string
	hash   string
}

func deriveKey(password string, key scryptKey) (string, error) {
	pass := []byte(password)
	salt, err := hex.DecodeString(key.salt)
	if err != nil {
		return "", err
	}

	hash, err := scrypt.Key(pass, salt, key.N, key.r, key.p, key.keyLen)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash), nil
}

func parseKey(str string) (scryptKey, error) {
	c := strings.Split(str, "$")
	if len(c) != 6 {
		return scryptKey{}, errInvalidKey
	}

	keyMap := make(map[string]string)
	for _, kvStr := range c {
		kv := strings.Split(kvStr, "=")
		if len(kv) != 2 {
			return scryptKey{}, errInvalidKey
		}

		keyMap[kv[0]] = kv[1]
	}

	if alg, ok := keyMap["alg"]; !ok || alg != "scrypt" {
		return scryptKey{}, errInvalidKey
	}

	N, err := getInt("N", keyMap)
	if err != nil || N%2 != 0 {
		return scryptKey{}, errInvalidKey
	}

	r, err := getInt("r", keyMap)
	if err != nil {
		return scryptKey{}, errInvalidKey
	}

	p, err := getInt("p", keyMap)
	if err != nil {
		return scryptKey{}, errInvalidKey
	}

	keyLen, err := getInt("keyLen", keyMap)
	if err != nil {
		return scryptKey{}, errInvalidKey
	}

	hash, ok := keyMap["hash"]
	if !ok || hash == "" || keyLen*2 != len(hash) {
		return scryptKey{}, errInvalidKey
	}

	return scryptKey{
		N:      N,
		p:      p,
		r:      r,
		keyLen: keyLen,
		hash:   hash,
	}, nil
}

func getInt(key string, keyMap map[string]string) (int, error) {
	str, ok := keyMap[key]
	if !ok || str == "" {
		return 0, errInvalidKey
	}

	val, err := strconv.Atoi(str)
	if err != nil || val == 0 {
		return 0, errInvalidKey
	}

	return val, nil
}
