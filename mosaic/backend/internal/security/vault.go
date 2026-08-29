// Package security implements the Secrets Vault (encrypted credential
// storage for Database/API connections) and the Script Node sandbox
// permission model. Credentials are never written to disk in plain text.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Vault stores secrets (API keys, tokens, DB passwords) encrypted with a
// key derived from a master passphrase. In a packaged Tauri build, the
// master key itself should come from the OS credential store (Keychain /
// Credential Manager / Secret Service) — MasterKeyFromOS is the seam where
// the Rust shell supplies that key over the sidecar bridge; this file
// implements the pure-Go AES-GCM envelope around whatever key it's given.
type Vault struct {
	mu     sync.RWMutex
	key    [32]byte
	secret map[string]string // key -> base64(nonce || ciphertext)
}

// NewVault derives a 256-bit key from a passphrase via SHA-256. In
// production the passphrase is the OS-keychain-backed master key, never a
// user-typed password stored anywhere.
func NewVault(passphrase string) *Vault {
	return &Vault{key: sha256.Sum256([]byte(passphrase)), secret: map[string]string{}}
}

// Set encrypts and stores a secret under key (e.g. "conn:postgres-prod:password").
func (v *Vault) Set(key, plaintext string) error {
	block, err := aes.NewCipher(v.key[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	v.mu.Lock()
	defer v.mu.Unlock()
	v.secret[key] = base64.StdEncoding.EncodeToString(ciphertext)
	return nil
}

// Get decrypts and returns a stored secret.
func (v *Vault) Get(key string) (string, error) {
	v.mu.RLock()
	enc, ok := v.secret[key]
	v.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("security: no secret stored for %q", key)
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(v.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("security: corrupt secret payload")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("security: failed to decrypt secret: %w", err)
	}
	return string(plain), nil
}

// Delete removes a stored secret.
func (v *Vault) Delete(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.secret, key)
}

// Keys lists stored secret identifiers (never values) for the Connections
// settings screen.
func (v *Vault) Keys() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]string, 0, len(v.secret))
	for k := range v.secret {
		out = append(out, k)
	}
	return out
}
