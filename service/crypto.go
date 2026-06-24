package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"metapi/aggrsite/config"
)

func getCipherKey() []byte {
	// Use AuthToken as the base for the encryption key, hashed to 32 bytes for AES-256
	hash := sha256.Sum256([]byte(config.C.AuthToken))
	return hash[:]
}

// EncryptPassword encrypts a plaintext password using AES-GCM
func EncryptPassword(plaintext string) string {
	if plaintext == "" {
		return ""
	}

	block, err := aes.NewCipher(getCipherKey())
	if err != nil {
		return ""
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return ""
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext)
}

// DecryptPassword decrypts a cipher text back to plaintext
func DecryptPassword(encryptedHex string) string {
	if encryptedHex == "" {
		return ""
	}

	enc, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return ""
	}

	block, err := aes.NewCipher(getCipherKey())
	if err != nil {
		return ""
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}

	nonceSize := aesGCM.NonceSize()
	if len(enc) < nonceSize {
		return ""
	}

	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return ""
	}

	return string(plaintext)
}
