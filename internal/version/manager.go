package version

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/schollz/progressbar/v3"

	"omp-cli/internal/config"
)

func EnsureClients(cfg *config.AppConfig, versionID string, gamePath string, isQuiet bool) (sampDllPath string, ompDllPath string, err error) {
	expectedHash, ok := cfg.Checksums[versionID]
	if !ok {
		return "", "", fmt.Errorf("unsupported version %s", versionID)
	}

	configDir, _ := os.UserConfigDir()
	cacheDir := filepath.Join(configDir, "omp-cli", "cache")
	os.MkdirAll(cacheDir, 0755)

	sampDllPath = filepath.Join(gamePath, "samp.dll")
	ompDllPath = filepath.Join(gamePath, "omp-client.dll")

	skipChecksum := cfg.Security.DisableChecksumVerification

	if skipChecksum || !checkSHA256(sampDllPath, expectedHash) {
		if !isQuiet && !skipChecksum {
			fmt.Printf("[Dependency Manager] Extracting SA-MP %s environment...\n", versionID)
		}
		archivePath := filepath.Join(cacheDir, "samp_clients.7z")

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			if !isQuiet {
				fmt.Printf("Downloading %s\n", cfg.Downloads.AssetsURL)
			}
			if err := downloadWithProgress(cfg.Downloads.AssetsURL, archivePath, isQuiet); err != nil {
				return "", "", fmt.Errorf("failed to download clients archive: %v", err)
			}
		}

		if err := extractSampClientEnvironment(archivePath, versionID, gamePath); err != nil {
			return "", "", fmt.Errorf("failed to extract SA-MP environment: %v", err)
		}

		if !skipChecksum && !checkSHA256(sampDllPath, expectedHash) {
			return "", "", fmt.Errorf("integrity check failed after extraction (hash mismatch)")
		}
	}

	if _, err := os.Stat(ompDllPath); os.IsNotExist(err) {
		if !isQuiet {
			fmt.Println("[Dependency Manager] Downloading omp-client.dll...")
		}
		if err := downloadWithProgress(cfg.Downloads.OmpClientURL, ompDllPath, isQuiet); err != nil {
			if !isQuiet {
				fmt.Printf("[Warning] Failed to fetch open.mp client. Vanilla SA-MP will be used.\n")
			}
			return sampDllPath, "", nil
		}
	}

	return sampDllPath, ompDllPath, nil
}

func extractSampClientEnvironment(archive string, versionID string, destDir string) error {
	r, err := sevenzip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	destDirClean := filepath.Clean(destDir)
	destPrefix := destDirClean + string(os.PathSeparator)

	for _, f := range r.File {
		var extractPath string
		normalizedName := strings.ReplaceAll(f.Name, "\\", "/")

		if strings.HasPrefix(normalizedName, "shared/") {
			relativePath := strings.TrimPrefix(normalizedName, "shared/")
			extractPath = filepath.Join(destDirClean, relativePath)
		} else if normalizedName == versionID+"/samp.dll" {
			extractPath = filepath.Join(destDirClean, "samp.dll")
		} else {
			continue
		}

		if !strings.HasPrefix(filepath.Clean(extractPath), destPrefix) && filepath.Clean(extractPath) != destDirClean {
			return fmt.Errorf("illegal file path: %s", extractPath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(extractPath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(extractPath), 0755)

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open %s in archive: %v", f.Name, err)
		}

		out, err := os.Create(extractPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("failed to create file %s: %v", extractPath, err)
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("failed to write %s: %v", extractPath, err)
		}
	}

	return nil
}

func checkSHA256(filePath string, expectedHash string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}

	return hex.EncodeToString(h.Sum(nil)) == expectedHash
}

func downloadWithProgress(url string, dest string, isQuiet bool) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad HTTP status: %d", resp.StatusCode)
	}

	tempDest := dest + ".tmp"
	out, err := os.Create(tempDest)
	if err != nil {
		return err
	}

	defer func() {
		out.Close()
		if err != nil {
			os.Remove(tempDest)
		}
	}()

	var writer io.Writer = out
	if !isQuiet {
		bar := progressbar.DefaultBytes(resp.ContentLength, "Downloading")
		writer = io.MultiWriter(out, bar)
	}

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return err
	}

	out.Close()
	return os.Rename(tempDest, dest)
}
