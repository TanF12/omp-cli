package i18n

import "os"

type Lang string

const (
	EN Lang = "en"
	PT Lang = "pt"
)

var translations = map[Lang]map[string]string{
	EN: {
		"usage":       "Usage: omp-cli <command> [args]\nCommands: query, launch, config",
		"launching":   "Starting open.mp via core...",
		"success":     "Game launched successfully!",
		"err_conn":    "Server is unreachable or offline.",
		"cfg_saved":   "Configuration saved successfully.",
		"cfg_req_inj": "Injector path is not configured. Please run: omp-cli config set-injector <path>",
		"cfg_current": "Current configuration:",
	},
	PT: {
		"usage":       "Uso: omp-cli <comando> [args]\nComandos: query, launch, config",
		"launching":   "Iniciando open.mp...",
		"success":     "Jogo iniciado com sucesso!",
		"err_conn":    "Servidor inalcançável ou offline.",
		"cfg_saved":   "Configuração salva com sucesso.",
		"cfg_req_inj": "O caminho do injetor não está configurado. Use: omp-cli config set-injector <caminho>",
		"cfg_current": "Configuração atual:",
	},
}

var currentLang = EN

func DetectLanguage() {
	lang := os.Getenv("LANG")
	if len(lang) >= 2 && lang[:2] == "pt" {
		currentLang = PT
	}
}

func T(key string) string {
	if val, ok := translations[currentLang][key]; ok {
		return val
	}
	return key
}
