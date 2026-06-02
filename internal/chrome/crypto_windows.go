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
// The key is stored in JSON at os_crypt.encrypted_key, base64-encoded
// with a "DPAPI" prefix. After stripping the prefix and base64-decoding,
// it is decrypted with DPAPI to produce the 256-bit AES key.
func readEncryptionKey(localStatePath string) ([]byte, error) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("chrome: read Local State: %w", err)
	}

	var state struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("chrome: parse Local State: %w", err)
	}

	if state.OSCrypt.EncryptedKey == "" {
		return nil, fmt.Errorf("chrome: os_crypt.encrypted_key not found in Local State")
	}

	raw, err := base64.StdEncoding.DecodeString(state.OSCrypt.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("chrome: base64 decode encrypted_key: %w", err)
	}

	const dpapiPrefix = "DPAPI"
	if len(raw) < len(dpapiPrefix) || string(raw[:len(dpapiPrefix)]) != dpapiPrefix {
		return nil, fmt.Errorf("chrome: unexpected encrypted_key prefix (expected 'DPAPI')")
	}

	key, err := decryptDPAPI(raw[len(dpapiPrefix):])
	if err != nil {
		return nil, fmt.Errorf("chrome: DPAPI decrypt key: %w", err)
	}

	if len(key) != 256/8 {
		return nil, fmt.Errorf("chrome: unexpected key length %d (expected 32)", len(key))
	}

	return key, nil
}

// decryptCookieValue decrypts a Chrome cookie value using the given AES key.
//
// Chrome cookie value formats:
//   - v10 prefix (3 bytes): AES-128-CBC, IV (16 bytes) + ciphertext
//   - v11 prefix (3 bytes): AES-256-GCM, nonce (12 bytes) + ciphertext + tag
func decryptCookieValue(encrypted []byte, key []byte) ([]byte, error) {
	if len(encrypted) < 4 {
		return nil, fmt.Errorf("chrome: cookie value too short (len=%d)", len(encrypted))
	}

	prefix := string(encrypted[:3])
	payload := encrypted[3:]

	switch prefix {
	case "v10":
		return decryptAES128CBC(key[:16], payload)
	case "v11":
		return decryptAES256GCM(key, payload)
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

// decryptAES256GCM decrypts using AES-256-GCM (Chrome v11 format).
// payload: nonce (12 bytes) + ciphertext + AEAD tag.
func decryptAES256GCM(key, payload []byte) ([]byte, error) {
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

	return gcm.Open(nil, nonce, ciphertext, nil)
}
