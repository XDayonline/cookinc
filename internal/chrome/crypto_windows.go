// Package chrome provides Windows DPAPI decryption for Chrome cookies.
//
// Build constraint: windows only.
package chrome

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	crypt32           = syscall.NewLazyDLL("crypt32.dll")
	procCryptUnprotect = crypt32.NewProc("CryptUnprotectData")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

// decryptDPAPI decrypts a blob using Win32 CryptUnprotectData.
func decryptDPAPI(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("DPAPI: empty input")
	}

	var inBlob dataBlob
	var outBlob dataBlob

	inBlob.cbData = uint32(len(data))
	inBlob.pbData = &data[0]

	ret, _, err := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, // pDataEntropy
		0, // pOptionalEntropy
		0, // pvReserved
		0, // pPromptStruct
		0, // dwFlags
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("DPAPI: CryptUnprotectData failed: %w", err)
	}

	result := make([]byte, outBlob.cbData)
	copy(result, unsafe.Slice(outBlob.pbData, outBlob.cbData))

	syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(outBlob.pbData))))

	return result, nil
}

// readEncryptionKey reads the Chrome AES encryption key from Local State.
//
// For Chrome < 127: os_crypt.encrypted_key (DPAPI-wrapped AES key).
// For Chrome >= 127: os_crypt.app_bound_encrypted_key (App-Bound key)
// is used for v20 cookies; the legacy encrypted_key is used for v10/v11.
//
// Both values are base64-encoded with a prefix ("DPAPI" or "APBB").
func readEncryptionKey(localStatePath string) ([]byte, error) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("chrome: read Local State: %w", err)
	}

	var state struct {
		OSCrypt struct {
			EncryptedKey        string `json:"encrypted_key"`
			AppBoundEncryptedKey string `json:"app_bound_encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("chrome: parse Local State: %w", err)
	}

	// Try App-Bound key first (Chrome 127+)
	if state.OSCrypt.AppBoundEncryptedKey != "" {
		key, err := decryptWrappedKey(state.OSCrypt.AppBoundEncryptedKey, "APBB")
		if err == nil && len(key) == 32 {
			return key, nil
		}
		// Fall through to legacy key
	}

	if state.OSCrypt.EncryptedKey == "" {
		return nil, fmt.Errorf("chrome: no encryption key found in Local State")
	}

	key, err := decryptWrappedKey(state.OSCrypt.EncryptedKey, "DPAPI")
	if err != nil {
		return nil, fmt.Errorf("chrome: decrypt key: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("chrome: unexpected key length %d (expected 32)", len(key))
	}

	return key, nil
}

// decryptWrappedKey base64-decodes a key that has the given prefix, then
// DPAPI-decrypts the remaining bytes.
func decryptWrappedKey(encoded, prefix string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	if len(raw) < len(prefix) || string(raw[:len(prefix)]) != prefix {
		return nil, fmt.Errorf("unexpected prefix %q (expected %q)", string(raw[:min(len(raw), len(prefix))]), prefix)
	}

	return decryptDPAPI(raw[len(prefix):])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// decryptCookieValue decrypts a Chrome cookie encrypted value using the
// given AES key. For v20 (Chrome 127+), the cookie name is needed as
// additional authenticated data (AAD).
//
// Chrome formats:
//   - v10: AES-128-CBC, IV (16) + ciphertext
//   - v11: AES-256-GCM, nonce (12) + ciphertext + tag
//   - v20: AES-256-GCM (App-Bound Encryption),
//     nonce (12) + ciphertext + tag, AAD = cookie name
func decryptCookieValue(encrypted []byte, key []byte, cookieName string) ([]byte, error) {
	if len(encrypted) < 4 {
		return nil, fmt.Errorf("chrome: cookie value too short (len=%d)", len(encrypted))
	}

	prefix := string(encrypted[:3])
	payload := encrypted[3:]

	switch prefix {
	case "v10":
		return decryptAES128CBC(key[:16], payload)
	case "v11":
		return decryptAES256GCM(key, payload, nil)
	case "v20":
		// Try with AAD (cookie name) first, then fall back to no AAD.
		// Chrome 127+ ABE uses a service-managed key, but some versions
		// still use the DPAPI-decrypted key with different AAD.
		result, err := decryptAES256GCM(key, payload, []byte(cookieName))
		if err != nil {
			result, err = decryptAES256GCM(key, payload, nil)
		}
		if err != nil {
			return nil, err
		}
		return result, nil
	default:
		return nil, fmt.Errorf("chrome: unknown cookie value prefix %q", prefix)
	}
}

// decryptAES128CBC decrypts using AES-128-CBC (Chrome v10 format).
// payload: IV (16 bytes) + ciphertext (PKCS#7 padded).
func decryptAES128CBC(key, payload []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("chrome: AES-128-CBC cipher: %w", err)
	}

	if len(payload) < aes.BlockSize {
		return nil, fmt.Errorf("chrome: AES-128-CBC payload too short (len=%d)", len(payload))
	}

	iv := payload[:aes.BlockSize]
	ciphertext := payload[aes.BlockSize:]

	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("chrome: AES-128-CBC empty ciphertext")
	}

	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)

	padding := int(plaintext[len(plaintext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return nil, fmt.Errorf("chrome: AES-128-CBC invalid PKCS#7 padding %d", padding)
	}
	return plaintext[:len(plaintext)-padding], nil
}

// decryptAES256GCM decrypts using AES-256-GCM (Chrome v11/v20 format).
// payload: nonce (12 bytes) + ciphertext + AEAD tag.
// aad is additional authenticated data (nil for v11, cookie name for v20).
func decryptAES256GCM(key, payload, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("chrome: AES-256-GCM cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("chrome: AES-256-GCM mode: %w", err)
	}

	if len(payload) < gcm.NonceSize() {
		return nil, fmt.Errorf("chrome: AES-256-GCM payload too short (len=%d)", len(payload))
	}

	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]

	return gcm.Open(nil, nonce, ciphertext, aad)
}
