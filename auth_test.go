package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeriveKey(t *testing.T) {
	assert := assert.New(t)

	key := scryptKey{
		N:      16384,
		r:      8,
		p:      1,
		keyLen: 32,
		salt:   "478c1d403dec20707cf487f81c06d646",
		hash:   "b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739",
	}

	password := "625181dbfb5c6100cdacd97f3ba32ab4"
	hash, err := deriveKey(password, key)
	assert.NoError(err)
	assert.Equal(key.hash, hash)

	wrongPassword := "41a9f7554a439eb7a652cd23cf4c3f89"
	wrongHash, err := deriveKey(wrongPassword, key)
	assert.NoError(err)
	assert.NotEqual(key.hash, wrongHash)
}

func TestParseKey_ok(t *testing.T) {
	assert := assert.New(t)

	okKey := "alg=scrypt$N=16384$r=8$p=1$keyLen=32$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739"
	key, err := parseKey(okKey)
	assert.NoError(err)
	assert.Equal(16384, key.N)
	assert.Equal(8, key.r)
	assert.Equal(1, key.p)
	assert.Equal(32, key.keyLen)
	assert.Equal("b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739", key.hash)

	// Should not have been parsed.
	assert.Equal("", key.salt)
}

func TestParseKey_notok(t *testing.T) {
	assert := assert.New(t)

	noAlg := "N=16384$r=8$p=1$keyLen=32$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739"
	_, err := parseKey(noAlg)
	assert.Equal(errInvalidKey, err)

	wrongAlg := "alg=pbkdf2$N=16384$r=8$p=1$keyLen=32$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739"
	_, err = parseKey(wrongAlg)
	assert.Equal(errInvalidKey, err)

	emptyHash := "alg=scrypt$N=16384$r=8$p=1$keyLen=32$hash="
	_, err = parseKey(emptyHash)
	assert.Equal(errInvalidKey, err)

	emptyKeyLen := "alg=scrypt$N=16384$r=8$p=1$keyLen=wrong$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739"
	_, err = parseKey(emptyKeyLen)
	assert.Equal(errInvalidKey, err)

	missmatchKeyLen := "alg=scrypt$N=16384$r=8$p=1$keyLen=64$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739"
	_, err = parseKey(missmatchKeyLen)
	assert.Equal(errInvalidKey, err)

	unevenN := "alg=scrypt$N=3$r=8$p=1$keyLen=32$hash=b8059f5d26826ef3af0faa424a8fc0f51f80bd62aa46ada056f7174e08a69739"
	_, err = parseKey(unevenN)
	assert.Equal(errInvalidKey, err)
}


