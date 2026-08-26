// Package hash provides hashing utilities for computing content hashes
// of configuration data, which are used for version comparison and ETag generation.
package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// MD5 computes the MD5 hash of a string.
func MD5(data string) string {
	h := md5.Sum([]byte(data))
	return hex.EncodeToString(h[:])
}

// SHA1 computes the SHA-1 hash of a string.
func SHA1(data string) string {
	h := sha1.Sum([]byte(data))
	return hex.EncodeToString(h[:])
}

// SHA256 computes the SHA-256 hash of a string.
func SHA256(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// MapHash computes a deterministic hash of a map[string]string.
// The keys are sorted before hashing to ensure the same map always
// produces the same hash regardless of iteration order.
func MapHash(m map[string]string) string {
	if len(m) == 0 {
		return MD5("{}")
	}
	// Sort keys for deterministic hashing
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(m[k])
		sb.WriteString(";")
	}
	return MD5(sb.String())
}

// StructHash computes a hash of any value by JSON-marshaling it first.
func StructHash(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal value: %w", err)
	}
	return SHA256(string(data)), nil
}

// ETag generates an ETag value from a hash string.
func ETag(hash string) string {
	return fmt.Sprintf("\"%s\"", hash)
}

// VerifyETag checks if an If-None-Match header value matches the given hash.
func VerifyETag(ifNoneMatch, hash string) bool {
	expected := ETag(hash)
	return ifNoneMatch == expected
}

// Combine concatenates multiple hashes into a single hash.
func Combine(hashes ...string) string {
	combined := strings.Join(hashes, "|")
	return MD5(combined)
}

// EmptyHash returns the hash of an empty value.
func EmptyHash() string {
	return MD5("")
}

// IsEmptyHash checks if a hash represents an empty value.
func IsEmptyHash(hash string) bool {
	return hash == EmptyHash()
}
