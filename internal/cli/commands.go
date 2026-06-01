package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"omp-cli/internal/config"
	"omp-cli/internal/core"
	"omp-cli/internal/i18n"
	"omp-cli/internal/tui"
	"omp-cli/internal/version"
)

func Execute() error {
	cfg, err := config.Load()
	if err == nil {
		i18n.InitLang(cfg.Language)
	}

	if len(os.Args) < 2 {
		return runInteractiveUI()
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "ui":
		return runInteractiveUI()
	case "query":
		return runQuery(args)
	case "launch":
		return runLaunch(args)
	case "config":
		return runConfig(args)
	default:
		fmt.Println(i18n.T("usage"))
		return nil
	}
}

func runInteractiveUI() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	if cfg.InjectorPath == "" || cfg.GamePath == "" {
		return fmt.Errorf("%s\nInjector Path and Game Path must be configured.", i18n.T("cfg_req_inj"))
	}

	var servers []core.OpenMpServer
	if cfg.MasterListURL != "" && strings.ToLower(cfg.MasterListURL) != "disabled" {
		fmt.Println(i18n.T("fetching_list"))
		fetched, err := core.FetchServersAPI(cfg.MasterListURL)
		if err != nil {
			fmt.Printf("Warning: Failed to fetch API (%v)\n", err)
		} else {
			servers = fetched
		}
	}

	launcher := func(ip string, port uint16, name string, ver string, password string) error {
		absGamePath, _ := filepath.Abs(cfg.GamePath)

		sampDll, ompDll, err := version.EnsureClients(cfg, ver, absGamePath, true)
		if err != nil {
			return fmt.Errorf("dependency error: %v", err)
		}

		var ompDllPtr *string
		if ompDll != "" {
			ompDllPtr = &ompDll
		}

		var passPtr *string
		if password != "" {
			passPtr = &password
		}

		launchCfg := core.LaunchConfig{
			Host:            ip,
			Port:            port,
			Name:            name,
			Password:        passPtr,
			GamePath:        absGamePath,
			DllPath:         sampDll,
			OmpDllPath:      ompDllPtr,
			IsWine:          cfg.IsWine,
			InjectorExePath: cfg.InjectorPath,
		}

		return core.LaunchGame(launchCfg)
	}

	p := tea.NewProgram(tui.InitialModel(cfg, servers, launcher), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("UI crashed: %v", err)
	}
	return nil
}

func runQuery(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: omp-cli query <ip:port>")
	}
	parts := strings.Split(args[0], ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid format. use IP:PORT")
	}
	port, _ := strconv.ParseUint(parts[1], 10, 16)

	info, err := core.QueryServer(parts[0], uint16(port))
	if err != nil {
		return fmt.Errorf("%s : %v", i18n.T("err_conn"), err)
	}

	fmt.Printf("Hostname: %s\nPlayers: %d/%d\nGamemode: %s\nPing: %dms\n",
		info.Hostname, info.Players, info.MaxPlayers, info.Gamemode, info.PingMs)
	return nil
}

func runConfig(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: omp-cli config [setup | view]")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch args[0] {
	case "setup":
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("=== OMP-CLI Configuration Setup ===")
		fmt.Println("(Press Enter to keep current value)")

		fmt.Printf("\nInjector Path (current: %s): ", cfg.InjectorPath)
		inj, _ := reader.ReadString('\n')
		if inj = strings.TrimSpace(inj); inj != "" {
			cfg.InjectorPath, _ = filepath.Abs(inj)
		}

		fmt.Printf("Game Path (current: %s): ", cfg.GamePath)
		game, _ := reader.ReadString('\n')
		if game = strings.TrimSpace(game); game != "" {
			cfg.GamePath, _ = filepath.Abs(game)
		}

		fmt.Printf("Default Nickname (current: %s): ", cfg.DefaultName)
		name, _ := reader.ReadString('\n')
		if name = strings.TrimSpace(name); name != "" {
			cfg.DefaultName = name
		}

		fmt.Printf("Use Wine (true/false)? (current: %v): ", cfg.IsWine)
		wine, _ := reader.ReadString('\n')
		if wine = strings.TrimSpace(wine); wine != "" {
			if w, err := strconv.ParseBool(wine); err == nil {
				cfg.IsWine = w
			}
		}

		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Println("\n" + i18n.T("cfg_saved"))

	case "view":
		fmt.Println(i18n.T("cfg_current"))
		fmt.Printf("Injector Path: %s\nGame Path: %s\nDefault Name: %s\nUse Wine: %v\nDefault Version: %s\nAPI URL: %s\nLanguage: %s\n",
			cfg.InjectorPath, cfg.GamePath, cfg.DefaultName, cfg.IsWine, cfg.DefaultVersion, cfg.MasterListURL, cfg.Language)
	default:
		return fmt.Errorf("unknown config command: %s", args[0])
	}
	return nil
}

func runLaunch(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	if cfg.InjectorPath == "" {
		return fmt.Errorf(i18n.T("cfg_req_inj"))
	}

	flags := flag.NewFlagSet("launch", flag.ExitOnError)
	ip := flags.String("ip", "127.0.0.1", "Server IP")
	port := flags.Int("port", 7777, "Server Port")
	name := flags.String("name", cfg.DefaultName, "Nickname")
	path := flags.String("path", cfg.GamePath, "GTA Directory Path")
	ver := flags.String("version", cfg.DefaultVersion, "SA-MP Version (e.g. 0.3.7-R5)")
	pwd := flags.String("password", "", "Server Password (Optional)")
	wine := flags.Bool("wine", cfg.IsWine, "Use Wine Wrapper (Linux)")
	noOmp := flags.Bool("no-omp", false, "Disable omp-client injection")
	flags.Parse(args)

	if *path == "" {
		return fmt.Errorf("game path is not provided via flag or config")
	}

	absGamePath, _ := filepath.Abs(*path)
	fmt.Println("Verifying DLLs...")

	sampDll, ompDll, err := version.EnsureClients(cfg, *ver, absGamePath, false)
	if err != nil {
		return fmt.Errorf("failed dependency check: %v", err)
	}

	var ompDllPtr *string
	if !*noOmp && ompDll != "" {
		ompDllPtr = &ompDll
	}

	var passPtr *string
	if *pwd != "" {
		passPtr = pwd
	}

	fmt.Println(i18n.T("launching"))
	launchCfg := core.LaunchConfig{
		Host:            *ip,
		Port:            uint16(*port),
		Name:            *name,
		Password:        passPtr,
		GamePath:        absGamePath,
		DllPath:         sampDll,
		OmpDllPath:      ompDllPtr,
		IsWine:          *wine,
		InjectorExePath: cfg.InjectorPath,
	}

	if err := core.LaunchGame(launchCfg); err != nil {
		return fmt.Errorf("core error: %v", err)
	}

	fmt.Println(i18n.T("success"))
	return nil
}
