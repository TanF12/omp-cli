package i18n

import (
	"os"
	"strings"
)

type Lang string

const (
	EN Lang = "en"
	PT Lang = "pt"
)

var translations = map[Lang]map[string]string{
	EN: {
		"usage":          "Usage: omp-cli <command> [args]\nCommands: query, launch, config",
		"launching":      "Starting open.mp via core...",
		"success":        "Game launched successfully!",
		"err_conn":       "Server is unreachable or offline.",
		"cfg_saved":      "Configuration saved successfully.",
		"cfg_req_inj":    "Injector path is not configured. Please run: omp-cli config setup",
		"cfg_current":    "Current configuration:",
		"fetching_list":  "Fetching master list...",
		"global_servers": "Global Servers",
		"favourites":     "Favourites",
		"no_servers":     "No servers found.",
		"select_server":  "Select a server to view diagnostics.",
		"offline":        "OFFLINE",
		"online":         "ONLINE",
		"players":        "Players",
		"ping":           "Ping",
		"name":           "Name",
		"status":         "CURRENT STATUS",
		"gamemode":       "GAMEMODE",
		"language":       "LANGUAGE",
		"rules":          "SERVER RULES",
		"bookmarked":     "Bookmarked",
		"pass":           "Pass",
		"rcon":           "RCON",
		"conn_failure":   "CONNECTION FAILURE",
		"init":           "Initialising...",
	},
	PT: {
		"usage":          "Uso: omp-cli <comando> [args]\nComandos: query, launch, config",
		"launching":      "Iniciando open.mp...",
		"success":        "Jogo iniciado com sucesso!",
		"err_conn":       "Servidor inalcançável ou offline.",
		"cfg_saved":      "Configuração salva com sucesso.",
		"cfg_req_inj":    "O caminho do injetor não está configurado. Use: omp-cli config setup",
		"cfg_current":    "Configuração atual:",
		"fetching_list":  "Buscando lista mestre...",
		"global_servers": "Servidores Globais",
		"favourites":     "Favoritos",
		"no_servers":     "Nenhum servidor encontrado.",
		"select_server":  "Selecione um servidor para ver os diagnósticos.",
		"offline":        "OFFLINE",
		"online":         "ONLINE",
		"players":        "Jogadores",
		"ping":           "Ping",
		"name":           "Nome",
		"status":         "STATUS ATUAL",
		"gamemode":       "MODO DE JOGO",
		"language":       "IDIOMA",
		"rules":          "REGRAS DO SERVIDOR",
		"bookmarked":     "Favoritado",
		"pass":           "Senha",
		"rcon":           "RCON",
		"conn_failure":   "FALHA NA CONEXÃO",
		"init":           "Inicializando...",
	},
}

var currentLang = EN

func InitLang(cfgLang string) {
	cfgLang = strings.ToLower(cfgLang)
	if cfgLang == "pt" {
		currentLang = PT
	} else if cfgLang == "en" {
		currentLang = EN
	} else {
		lang := os.Getenv("LANG")
		if len(lang) >= 2 && lang[:2] == "pt" {
			currentLang = PT
		}
	}
}

func T(key string) string {
	if val, ok := translations[currentLang][key]; ok {
		return val
	}
	return key
}
