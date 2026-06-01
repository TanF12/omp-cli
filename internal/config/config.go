package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/ini.v1"
)

type KeybindConfig struct {
	Quit           string
	Up             string
	Down           string
	SwitchTab      string
	TogglePlayers  string
	Search         string
	AddIP          string
	Delete         string
	SwitchVersion  string
	SetPassword    string
	SetRcon        string
	ChangeName     string
	ToggleBookmark string
	Import         string
	ChangeSort     string
	Enter          string
}

type DownloadConfig struct {
	AssetsURL    string
	OmpClientURL string
}

type SecurityConfig struct {
	DisableChecksumVerification bool
	SavePasswords               bool
	EncryptPasswords            bool
}

type AppConfig struct {
	InjectorPath   string
	GamePath       string
	DefaultName    string
	IsWine         bool
	DefaultVersion string
	MasterListURL  string
	Language       string
	Keybinds       KeybindConfig
	Downloads      DownloadConfig
	Security       SecurityConfig
	Checksums      map[string]string
}

var mu sync.Mutex

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	dir := filepath.Dir(exe)
	if strings.Contains(dir, "go-build") || strings.Contains(dir, "T/go-build") {
		if cwd, err := os.Getwd(); err == nil {
			return cwd
		}
	}
	return dir
}

func getIniPath() string {
	return filepath.Join(getExeDir(), "omp-cli.ini")
}

func loadIniFile() *ini.File {
	file, err := ini.Load(getIniPath())
	if err != nil {
		return ini.Empty()
	}
	return file
}

func saveIniFile(file *ini.File) error {
	return file.SaveTo(getIniPath())
}

func defaultKeybinds() KeybindConfig {
	return KeybindConfig{
		Quit:           "q, ctrl+c",
		Up:             "up, k",
		Down:           "down, j",
		SwitchTab:      "tab, right, left",
		TogglePlayers:  "c",
		Search:         "f",
		AddIP:          "a",
		Delete:         "d, delete",
		SwitchVersion:  "v",
		SetPassword:    "p",
		SetRcon:        "r",
		ChangeName:     "n",
		ToggleBookmark: "b",
		Import:         "i",
		ChangeSort:     "s",
		Enter:          "enter",
	}
}

func Load() (*AppConfig, error) {
	mu.Lock()
	defer mu.Unlock()

	file := loadIniFile()
	cfg := &AppConfig{
		DefaultName:    "Player",
		DefaultVersion: "0.3.7-R5",
		MasterListURL:  "https://api.open.mp/servers/",
		Language:       "auto",
		Keybinds:       defaultKeybinds(),
		Downloads: DownloadConfig{
			AssetsURL:    "https://assets.open.mp/samp_clients.7z",
			OmpClientURL: "https://assets.open.mp/omp-client.dll",
		},
		Security: SecurityConfig{
			DisableChecksumVerification: false,
			SavePasswords:               true,
			EncryptPasswords:            true,
		},
		Checksums: map[string]string{
			"0.3.7-R1":   "7e30f3c9cd99d5e2932410f486e8139affa2dad19bd65ad9c328f6a4071943f7",
			"0.3.7-R2":   "de07a850590a43d83a40f9251741c07d3d0d74a217d5a09cb498a32982e8315b",
			"0.3.7-R3":   "81d39af30eafe6176de82c57ef9d2a9eaa92268b18d7b17096f67919a9248040",
			"0.3.7-R3-1": "9c9b2cc31a4ced6967420b1880c096b5c4e7630e227aa379be4019c21b6fddc1",
			"0.3.7-R4":   "15db80c5c9e02e011f16509d081d1ce7c8526238200814ebc16ba1f4f9ff12ab",
			"0.3.7-R5":   "b72b5dbe725f81864ca3f78bc7063bda56cc05fc7188af822fa7a754432553a2",
			"0.3.DL":     "bccdb297464bd382625635be25585df07a8fa6668bc0015650708e3eb4ffcd4b",
		},
	}

	sec := file.Section("Settings")
	if sec.HasKey("InjectorPath") {
		cfg.InjectorPath = sec.Key("InjectorPath").String()
	}
	if sec.HasKey("GamePath") {
		cfg.GamePath = sec.Key("GamePath").String()
	}
	if sec.HasKey("DefaultName") && sec.Key("DefaultName").String() != "" {
		cfg.DefaultName = sec.Key("DefaultName").String()
	}
	if sec.HasKey("IsWine") {
		cfg.IsWine, _ = sec.Key("IsWine").Bool()
	}
	if sec.HasKey("DefaultVersion") && sec.Key("DefaultVersion").String() != "" {
		cfg.DefaultVersion = sec.Key("DefaultVersion").String()
	}
	if sec.HasKey("MasterListURL") && sec.Key("MasterListURL").String() != "" {
		cfg.MasterListURL = sec.Key("MasterListURL").String()
	}
	if sec.HasKey("Language") && sec.Key("Language").String() != "" {
		cfg.Language = sec.Key("Language").String()
	}

	dl := file.Section("Downloads")
	if dl.HasKey("AssetsURL") && dl.Key("AssetsURL").String() != "" {
		cfg.Downloads.AssetsURL = dl.Key("AssetsURL").String()
	}
	if dl.HasKey("OmpClientURL") && dl.Key("OmpClientURL").String() != "" {
		cfg.Downloads.OmpClientURL = dl.Key("OmpClientURL").String()
	}

	secu := file.Section("Security")
	if secu.HasKey("DisableChecksumVerification") {
		cfg.Security.DisableChecksumVerification, _ = secu.Key("DisableChecksumVerification").Bool()
	}
	if secu.HasKey("SavePasswords") {
		cfg.Security.SavePasswords, _ = secu.Key("SavePasswords").Bool()
	}
	if secu.HasKey("EncryptPasswords") {
		cfg.Security.EncryptPasswords, _ = secu.Key("EncryptPasswords").Bool()
	}

	chk := file.Section("Checksums")
	if len(chk.Keys()) > 0 {
		cfg.Checksums = make(map[string]string)
		for _, key := range chk.Keys() {
			cfg.Checksums[key.Name()] = key.String()
		}
	}

	kb := file.Section("Keybinds")
	if kb.HasKey("Quit") && kb.Key("Quit").String() != "" {
		cfg.Keybinds.Quit = kb.Key("Quit").String()
	}
	if kb.HasKey("Up") && kb.Key("Up").String() != "" {
		cfg.Keybinds.Up = kb.Key("Up").String()
	}
	if kb.HasKey("Down") && kb.Key("Down").String() != "" {
		cfg.Keybinds.Down = kb.Key("Down").String()
	}
	if kb.HasKey("SwitchTab") && kb.Key("SwitchTab").String() != "" {
		cfg.Keybinds.SwitchTab = kb.Key("SwitchTab").String()
	}
	if kb.HasKey("TogglePlayers") && kb.Key("TogglePlayers").String() != "" {
		cfg.Keybinds.TogglePlayers = kb.Key("TogglePlayers").String()
	}
	if kb.HasKey("Search") && kb.Key("Search").String() != "" {
		cfg.Keybinds.Search = kb.Key("Search").String()
	}
	if kb.HasKey("AddIP") && kb.Key("AddIP").String() != "" {
		cfg.Keybinds.AddIP = kb.Key("AddIP").String()
	}
	if kb.HasKey("Delete") && kb.Key("Delete").String() != "" {
		cfg.Keybinds.Delete = kb.Key("Delete").String()
	}
	if kb.HasKey("SwitchVersion") && kb.Key("SwitchVersion").String() != "" {
		cfg.Keybinds.SwitchVersion = kb.Key("SwitchVersion").String()
	}
	if kb.HasKey("SetPassword") && kb.Key("SetPassword").String() != "" {
		cfg.Keybinds.SetPassword = kb.Key("SetPassword").String()
	}
	if kb.HasKey("SetRcon") && kb.Key("SetRcon").String() != "" {
		cfg.Keybinds.SetRcon = kb.Key("SetRcon").String()
	}
	if kb.HasKey("ChangeName") && kb.Key("ChangeName").String() != "" {
		cfg.Keybinds.ChangeName = kb.Key("ChangeName").String()
	}
	if kb.HasKey("ToggleBookmark") && kb.Key("ToggleBookmark").String() != "" {
		cfg.Keybinds.ToggleBookmark = kb.Key("ToggleBookmark").String()
	}
	if kb.HasKey("Import") && kb.Key("Import").String() != "" {
		cfg.Keybinds.Import = kb.Key("Import").String()
	}
	if kb.HasKey("ChangeSort") && kb.Key("ChangeSort").String() != "" {
		cfg.Keybinds.ChangeSort = kb.Key("ChangeSort").String()
	}
	if kb.HasKey("Enter") && kb.Key("Enter").String() != "" {
		cfg.Keybinds.Enter = kb.Key("Enter").String()
	}

	go Save(cfg)

	return cfg, nil
}

func Save(cfg *AppConfig) error {
	mu.Lock()
	defer mu.Unlock()

	file := loadIniFile()

	sec := file.Section("Settings")
	sec.Key("InjectorPath").SetValue(cfg.InjectorPath)
	sec.Key("GamePath").SetValue(cfg.GamePath)
	sec.Key("DefaultName").SetValue(cfg.DefaultName)
	sec.Key("IsWine").SetValue(fmt.Sprintf("%v", cfg.IsWine))
	sec.Key("DefaultVersion").SetValue(cfg.DefaultVersion)
	sec.Key("MasterListURL").SetValue(cfg.MasterListURL)
	sec.Key("Language").SetValue(cfg.Language)

	dl := file.Section("Downloads")
	dl.Key("AssetsURL").SetValue(cfg.Downloads.AssetsURL)
	dl.Key("OmpClientURL").SetValue(cfg.Downloads.OmpClientURL)

	secu := file.Section("Security")
	secu.Key("DisableChecksumVerification").SetValue(fmt.Sprintf("%v", cfg.Security.DisableChecksumVerification))
	secu.Key("SavePasswords").SetValue(fmt.Sprintf("%v", cfg.Security.SavePasswords))
	secu.Key("EncryptPasswords").SetValue(fmt.Sprintf("%v", cfg.Security.EncryptPasswords))

	file.DeleteSection("Checksums")
	chk, _ := file.NewSection("Checksums")
	for k, v := range cfg.Checksums {
		chk.Key(k).SetValue(v)
	}

	kb := file.Section("Keybinds")
	kb.Key("Quit").SetValue(cfg.Keybinds.Quit)
	kb.Key("Up").SetValue(cfg.Keybinds.Up)
	kb.Key("Down").SetValue(cfg.Keybinds.Down)
	kb.Key("SwitchTab").SetValue(cfg.Keybinds.SwitchTab)
	kb.Key("TogglePlayers").SetValue(cfg.Keybinds.TogglePlayers)
	kb.Key("Search").SetValue(cfg.Keybinds.Search)
	kb.Key("AddIP").SetValue(cfg.Keybinds.AddIP)
	kb.Key("Delete").SetValue(cfg.Keybinds.Delete)
	kb.Key("SwitchVersion").SetValue(cfg.Keybinds.SwitchVersion)
	kb.Key("SetPassword").SetValue(cfg.Keybinds.SetPassword)
	kb.Key("SetRcon").SetValue(cfg.Keybinds.SetRcon)
	kb.Key("ChangeName").SetValue(cfg.Keybinds.ChangeName)
	kb.Key("ToggleBookmark").SetValue(cfg.Keybinds.ToggleBookmark)
	kb.Key("Import").SetValue(cfg.Keybinds.Import)
	kb.Key("ChangeSort").SetValue(cfg.Keybinds.ChangeSort)
	kb.Key("Enter").SetValue(cfg.Keybinds.Enter)

	return saveIniFile(file)
}

func (c *AppConfig) GetAvailableVersions() []string {
	var vers []string
	for k := range c.Checksums {
		vers = append(vers, k)
	}
	sort.Strings(vers)
	return vers
}
