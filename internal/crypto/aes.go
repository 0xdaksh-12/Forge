package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encrypt encrypts a plaintext string using AES-256-GCM.
// The key must be a hex-encoded 32-byte string.
func Encrypt(plaintext string, hexKey string) (ciphertextHex, nonceHex string, err error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", "", fmt.Errorf("invalid hex key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), hex.EncodeToString(nonce), nil
}

// Decrypt decrypts a ciphertext string using AES-256-GCM.
// The key must be a hex-encoded 32-byte string.
func Decrypt(ciphertextHex, nonceHex, hexKey string) (plaintext string, err error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("invalid hex key: %w", err)
	}

	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext hex: %w", err)
	}

	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return "", fmt.Errorf("invalid nonce hex: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintextBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintextBytes), nil
}
