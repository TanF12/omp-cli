package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Favouriteserver struct {
	RconPassword   string
	ServerPassword string
}

type FavouritesData struct {
	Servers map[string]Favouriteserver
}

func LoadFavourites() (*FavouritesData, error) {
	mu.Lock()
	defer mu.Unlock()

	file := loadIniFile()
	favs := &FavouritesData{Servers: make(map[string]Favouriteserver)}

	sec := file.Section("Favourites")
	for _, key := range sec.Keys() {
		k := key.Name()
		v := key.String()

		var s Favouriteserver
		if v != "" {
			parts := strings.SplitN(v, "|", 2)
			s.ServerPassword = parts[0]
			if len(parts) > 1 {
				s.RconPassword = parts[1]
			}
		}
		favs.Servers[k] = s
	}

	return favs, nil
}

func SaveFavourites(favs *FavouritesData) error {
	mu.Lock()
	defer mu.Unlock()

	file := loadIniFile()

	file.DeleteSection("Favourites")
	sec, _ := file.NewSection("Favourites")

	for k, v := range favs.Servers {
		val := v.ServerPassword
		if v.RconPassword != "" {
			val += "|" + v.RconPassword
		}
		sec.Key(k).SetValue(val)
	}

	return saveIniFile(file)
}

func getAesKey() ([]byte, error) {
	keyPath := filepath.Join(getExeDir(), "aes.key")

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.WriteFile(keyPath, key, 0600); err != nil {
			return nil, err
		}
		return key, nil
	}
	return os.ReadFile(keyPath)
}

func encryptAESBase64(plaintext string) (string, error) {
	key, err := getAesKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptAESBase64(encoded string) (string, error) {
	key, err := getAesKey()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func EncryptPassword(plaintext string, encrypt bool) string {
	if plaintext == "" {
		return ""
	}
	if encrypt {
		enc, err := encryptAESBase64(plaintext)
		if err == nil {
			return "ENC:" + enc
		}
	}
	return plaintext
}

func DecryptPassword(encoded string) string {
	if encoded == "" {
		return ""
	}

	toDecode := encoded
	if strings.HasPrefix(encoded, "ENC:") {
		toDecode = encoded[4:]
	}

	decrypted, err := decryptAESBase64(toDecode)
	if err == nil {
		return decrypted
	}

	if strings.HasPrefix(encoded, "ENC:") {
		return ""
	}
	return encoded
}

func ImportSAMPFavourites(favs *FavouritesData) int {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}

	path := filepath.Join(home, "Documents", "GTA San Andreas User Files", "SAMP", "USERDATA.DAT")
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	var fileId [4]byte
	if err := binary.Read(f, binary.LittleEndian, &fileId); err != nil {
		return 0
	}
	if string(fileId[:]) != "SAMP" {
		return 0
	}

	var fileVersion, serverCount uint32
	if err := binary.Read(f, binary.LittleEndian, &fileVersion); err != nil {
		return 0
	}
	if err := binary.Read(f, binary.LittleEndian, &serverCount); err != nil {
		return 0
	}

	imported := 0
	for i := uint32(0); i < serverCount; i++ {
		var ipLen, port, nameLen, passLen, rconLen uint32

		if err := binary.Read(f, binary.LittleEndian, &ipLen); err != nil {
			break
		}
		if ipLen > 256 {
			break
		}

		ipBuf := make([]byte, ipLen)
		if _, err := io.ReadFull(f, ipBuf); err != nil {
			break
		}

		if err := binary.Read(f, binary.LittleEndian, &port); err != nil {
			break
		}

		if err := binary.Read(f, binary.LittleEndian, &nameLen); err != nil {
			break
		}
		if nameLen > 1024 {
			break
		}
		if _, err := f.Seek(int64(nameLen), io.SeekCurrent); err != nil {
			break
		}

		if err := binary.Read(f, binary.LittleEndian, &passLen); err != nil {
			break
		}
		if passLen > 1024 {
			break
		}
		if _, err := f.Seek(int64(passLen), io.SeekCurrent); err != nil {
			break
		}

		if err := binary.Read(f, binary.LittleEndian, &rconLen); err != nil {
			break
		}
		if rconLen > 1024 {
			break
		}
		if _, err := f.Seek(int64(rconLen), io.SeekCurrent); err != nil {
			break
		}

		ip := string(ipBuf)
		if ip != "" {
			target := fmt.Sprintf("%s:%d", ip, port)
			if _, exists := favs.Servers[target]; !exists {
				favs.Servers[target] = Favouriteserver{}
				imported++
			}
		}
	}
	return imported
}
