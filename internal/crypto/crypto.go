package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	SaltSize   = 32
	KeySize    = 32 // AES-256
	NonceSize  = 12 // Standard GCM nonce
	Iterations = 100000
)

// Encrypt compresses data (implied, passed as tar.gz bytes) and encrypts it with AES-GCM
func Encrypt(data []byte, passphrase string) ([]byte, error) {
	// 1. Generate random salt
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// 2. Derive key from passphrase
	key := pbkdf2.Key([]byte(passphrase), salt, Iterations, KeySize, sha256.New)

	// 3. Create AES Cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher creation failed: %w", err)
	}

	// 4. Create GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm creation failed: %w", err)
	}

	// 5. Generate Nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce generation failed: %w", err)
	}

	// 6. Seal (Encrypt)
	// Output Format: [Salt (32)] + [Nonce (12)] + [Ciphertext + Tag]
	ciphertext := aesGCM.Seal(nil, nonce, data, nil)

	final := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	final = append(final, salt...)
	final = append(final, nonce...)
	final = append(final, ciphertext...)

	return final, nil
}

// Decrypt reverses the process
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	if len(data) < SaltSize+NonceSize {
		return nil, fmt.Errorf("invalid data: too short")
	}

	// 1. Extract Metadata
	salt := data[:SaltSize]
	nonce := data[SaltSize : SaltSize+NonceSize]
	ciphertext := data[SaltSize+NonceSize:]

	// 2. Re-derive Key
	key := pbkdf2.Key([]byte(passphrase), salt, Iterations, KeySize, sha256.New)

	// 3. Init Cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher failed: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm failed: %w", err)
	}

	// 4. Decrypt
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: incorrect password or corrupted file")
	}

	return plaintext, nil
}
