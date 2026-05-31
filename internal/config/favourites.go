package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FavoriteServer struct {
	RconPassword   string `json:"rcon_password,omitempty"`
	ServerPassword string `json:"server_password,omitempty"`
}

type FavoritesData struct {
	Servers map[string]FavoriteServer `json:"servers"`
}

func getFavoritesPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "omp-cli", "favorites.json"), nil
}

func LoadFavorites() (*FavoritesData, error) {
	path, err := getFavoritesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &FavoritesData{Servers: make(map[string]FavoriteServer)}, nil
	}

	var favs FavoritesData
	if err := json.Unmarshal(data, &favs); err != nil {
		return nil, err
	}
	if favs.Servers == nil {
		favs.Servers = make(map[string]FavoriteServer)
	}
	return &favs, nil
}

func SaveFavorites(favs *FavoritesData) error {
	path, err := getFavoritesPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(favs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func getAesKey() ([]byte, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	keyPath := filepath.Join(configDir, "omp-cli", "aes.key")

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

func EncryptAES(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
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

func DecryptAES(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
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

func ImportSAMPFavorites(favs *FavoritesData) int {
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
				favs.Servers[target] = FavoriteServer{}
				imported++
			}
		}
	}
	return imported
}
