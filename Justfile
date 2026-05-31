
REPO_URL := "https://github.com/TanF12/omp-launchlib/releases/latest/download"
CONFIG_DIR := if os() == "windows" { env_var("APPDATA") + "/omp-cli" } else { env_var("HOME") + "/.config/omp-cli" }
BIN_DIR := CONFIG_DIR + "/bin"

default:
    @echo "Available commands:"
    @echo "  just install   - Download dependencies, build CLI, set up config and shortcuts"
    @echo "  just build     - Just build the Go CLI"

#Creates directories
setup-dirs:
    @echo "=> Creating configuration directories in {{CONFIG_DIR}}..."
    mkdir -p "{{BIN_DIR}}"

#Downloads the compiled Rust binaries from GitHub
download-core: setup-dirs
    @echo "=> Downloading omp-injector.exe..."
    curl -L -o "{{BIN_DIR}}/omp-injector.exe" "{{REPO_URL}}/omp-injector.exe"
    
    @echo "=> Downloading omp-launchlib library..."
    @if [ "$(uname)" = "Linux" ]; then \
        curl -L -o "{{BIN_DIR}}/libomp_core.so" "{{REPO_URL}}/libomp_core.so"; \
    else \
        curl -L -o "{{BIN_DIR}}/omp_core.dll" "{{REPO_URL}}/omp_core.dll"; \
    fi

#Builds the Go binary
build:
    @echo "=> Building omp-cli..."
    go build -o omp-cli ./cmd/omp-cli

#Installation routine
install: download-core build
    @echo "=> Configuring omp-cli..."
    ./omp-cli config set-injector "{{BIN_DIR}}/omp-injector.exe"
    
    @if [ "$(uname)" = "Linux" ]; then \
        echo "=> Creating Linux Desktop Shortcut..."; \
        mkdir -p ~/.local/share/applications; \
        echo "[Desktop Entry]\nName=open.mp Launcher\nExec=$(pwd)/omp-cli ui\nTerminal=true\nType=Application\nCategories=Game;" > ~/.local/share/applications/omp-cli.desktop; \
        chmod +x ~/.local/share/applications/omp-cli.desktop; \
        ./omp-cli config set-wine true; \
        echo "=> Success! You can now launch 'open.mp Launcher' from your application menu."; \
    else \
        echo "=> Success! Execute ./omp-cli ui to play."; \
    fi