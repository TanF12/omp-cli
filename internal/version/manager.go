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
)

const (
	AssetURL     = "https://assets.open.mp/samp_clients.7z"
	OmpClientURL = "https://assets.open.mp/omp-client.dll"
)

type VersionInfo struct {
	ID       string
	Name     string
	Checksum string
}

var AvailableVersions = []string{
	"0.3.7-R1",
	"0.3.7-R2",
	"0.3.7-R3",
	"0.3.7-R3-1",
	"0.3.7-R4",
	"0.3.7-R5",
	"0.3.DL",
}

var SampVersions = map[string]VersionInfo{
	"0.3.7-R1":   {ID: "0.3.7-R1", Name: "SA-MP 0.3.7 R1", Checksum: "7e30f3c9cd99d5e2932410f486e8139affa2dad19bd65ad9c328f6a4071943f7"},
	"0.3.7-R2":   {ID: "0.3.7-R2", Name: "SA-MP 0.3.7 R2", Checksum: "de07a850590a43d83a40f9251741c07d3d0d74a217d5a09cb498a32982e8315b"},
	"0.3.7-R3":   {ID: "0.3.7-R3", Name: "SA-MP 0.3.7 R3", Checksum: "81d39af30eafe6176de82c57ef9d2a9eaa92268b18d7b17096f67919a9248040"},
	"0.3.7-R3-1": {ID: "0.3.7-R3-1", Name: "SA-MP 0.3.7 R3-1", Checksum: "9c9b2cc31a4ced6967420b1880c096b5c4e7630e227aa379be4019c21b6fddc1"},
	"0.3.7-R4":   {ID: "0.3.7-R4", Name: "SA-MP 0.3.7 R4", Checksum: "15db80c5c9e02e011f16509d081d1ce7c8526238200814ebc16ba1f4f9ff12ab"},
	"0.3.7-R5":   {ID: "0.3.7-R5", Name: "SA-MP 0.3.7 R5", Checksum: "b72b5dbe725f81864ca3f78bc7063bda56cc05fc7188af822fa7a754432553a2"},
	"0.3.DL":     {ID: "0.3.DL", Name: "SA-MP 0.3.DL", Checksum: "bccdb297464bd382625635be25585df07a8fa6668bc0015650708e3eb4ffcd4b"},
}

func EnsureClients(versionID string, gamePath string, isQuiet bool) (sampDllPath string, ompDllPath string, err error) {
	ver, ok := SampVersions[versionID]
	if !ok {
		return "", "", fmt.Errorf("unsupported version %s", versionID)
	}

	configDir, _ := os.UserConfigDir()
	cacheDir := filepath.Join(configDir, "omp-cli", "cache")
	os.MkdirAll(cacheDir, 0755)

	sampDllPath = filepath.Join(gamePath, "samp.dll")
	ompDllPath = filepath.Join(gamePath, "omp-client.dll")

	if !checkSHA256(sampDllPath, ver.Checksum) {
		if !isQuiet {
			fmt.Printf("[Dependency Manager] Extracting %s environment...\n", ver.Name)
		}
		archivePath := filepath.Join(cacheDir, "samp_clients.7z")

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			if !isQuiet {
				fmt.Printf("Downloading %s\n", AssetURL)
			}
			if err := downloadWithProgress(AssetURL, archivePath, isQuiet); err != nil {
				return "", "", fmt.Errorf("failed to download clients archive: %v", err)
			}
		}

		if err := extractSampClientEnvironment(archivePath, ver.ID, gamePath); err != nil {
			return "", "", fmt.Errorf("failed to extract SA-MP environment: %v", err)
		}

		if !checkSHA256(sampDllPath, ver.Checksum) {
			return "", "", fmt.Errorf("integrity check failed after extraction (hash mismatch)")
		}
	}

	if _, err := os.Stat(ompDllPath); os.IsNotExist(err) {
		if !isQuiet {
			fmt.Println("[Dependency Manager] Downloading omp-client.dll...")
		}
		if err := downloadWithProgress(OmpClientURL, ompDllPath, isQuiet); err != nil {
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
