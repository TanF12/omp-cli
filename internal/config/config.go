package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	InjectorPath   string `json:"injector_path"`
	GamePath       string `json:"game_path"`
	DefaultName    string `json:"default_name"`
	IsWine         bool   `json:"is_wine"`
	DefaultVersion string `json:"default_version"`
}

func getPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "omp-cli")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.json"), nil
}

func Load() (*AppConfig, error) {
	configPath, err := getPath()
	if err != nil {
		return &AppConfig{}, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AppConfig{DefaultName: "Player", DefaultVersion: "0.3.7-R5"}, nil
		}
		return nil, err
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.DefaultVersion == "" {
		cfg.DefaultVersion = "0.3.7-R5"
	}
	return &cfg, nil
}

func Save(cfg *AppConfig) error {
	configPath, err := getPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
