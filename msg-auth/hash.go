package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"

	"golang.org/x/crypto/sha3"
)

// HashAlgorithm is an enum of what different hashing algorithms are supported by the Hasher
type HashAlgorithm string

const (
	// MD5_128 uses the MD5 (128-bit) algorithm
	MD5_128 HashAlgorithm = "md5-128"
	// SHA1_160 uses the SHA-1 (160-bit) algorithm
	SHA1_160 HashAlgorithm = "sha1-160"
	// SHA2_256 uses the SHA-2 256-bit algorithm, commonly referred to as just sha256
	SHA2_256 HashAlgorithm = "sha2-256"
	// SHA2_512 uses the SHA-2 512-bit algorithm, commonly referred to as just sha512
	SHA2_512 HashAlgorithm = "sha2-512"
	// SHA3_256 uses the SHA-3 256-bit algorithm
	SHA3_256 HashAlgorithm = "sha3-256"
	// SHA3_512 uses the SHA-3 512-bit algorithm
	SHA3_512 HashAlgorithm = "sha3-512"
)

// CreateHashFunc is a function which returns Golang's hash.Hash objects
type CreateHashFunc func() hash.Hash

// hashers is a map describing the supported hash algorithms
var hashers = map[HashAlgorithm]CreateHashFunc{
	MD5_128:  md5.New,
	SHA1_160: sha1.New,
	SHA2_256: sha256.New,
	SHA2_512: sha512.New,
	SHA3_256: sha3.New256,
	SHA3_512: sha3.New512,
}

// SupportedHashAlgorithms returns the supported hash algorithms for this program
func SupportedHashAlgorithms() (algos []HashAlgorithm) {
	for algo := range hashers {
		algos = append(algos, algo)
	}
	return
}

// Hasher is an interface for hashing possibly prefixed data using various algorithms
type Hasher interface {
	// The io.Writer interface contains the following signature:
	// 	   Write(prefix []byte) (n int, err error)
	// which writes a "prefix" (or data to be hashed) into the buffer of the hashing algorithm.
	// This data is used for all invocations of Hash() during this object's lifetime
	io.Writer

	// Hash returns the digest of the hashing algorithm in "raw" bytes, possibly feeding "suffix" bytes into
	// the output before the calculation. In other words, the returned hash is H(prefix + suffix). The suffix
	// does not change the state of the object.
	Hash(suffix []byte) []byte

	// Size returns the amount of bytes returned by the Hash() function
	Size() uint8
}

// NewHasher returns a new Hasher for the given algorithm
func NewHasher(algo HashAlgorithm) (Hasher, error) {
	initFn, ok := hashers[algo]
	if !ok {
		return nil, fmt.Errorf("hash type does not exist: %d", algo)
	}

	return &hasher{
		initFn: initFn,
		algo:   algo,
		prefix: nil,
	}, nil
}

// hasher is the struct implementation of the Hasher interface
type hasher struct {
	initFn CreateHashFunc
	algo   HashAlgorithm
	prefix []byte
}

// Write writes a "prefix" (or data to be hashed) into the buffer of the hashing algorithm.
// This data is used for all invocations of Hash() during this object's lifetime
func (h *hasher) Write(prefix []byte) (n int, err error) {
	h.prefix = append(h.prefix, prefix...)
	n = len(prefix)
	return
}

// Hash returns the digest of the hashing algorithm in "raw" bytes, possibly feeding "suffix" bytes into
// the output before the calculation. In other words, the returned hash is H(prefix + suffix). The suffix
// does not change the state of the object.
func (h *hasher) Hash(suffix []byte) []byte {
	hashImpl := h.initFn()
	_, _ = hashImpl.Write(h.prefix)
	_, _ = hashImpl.Write(suffix)
	return hashImpl.Sum(nil)
}

// Size returns the amount of bytes returned by the Hash() function
func (h *hasher) Size() uint8 {
	return uint8(h.initFn().Size())
}
