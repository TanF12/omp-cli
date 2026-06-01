
REPO_URL := "https://github.com/TanF12/omp-launchlib/releases/latest/download"
CONFIG_DIR := if os() == "windows" { env_var("APPDATA") + "/omp-cli" } else { env_var("HOME") + "/.config/omp-cli" }
BIN_DIR := CONFIG_DIR + "/bin"
LOCAL_BIN := env_var("HOME") + "/.local/bin"

default:
    @echo "Available commands:"
    @echo "  just install   - Download dependencies, build CLI, set up config and shortcuts"
    @echo "  just build     - Just build the Go CLI"

#Creates directories
setup-dirs:
    @echo "=> Creating configuration directories in {{CONFIG_DIR}}..."
    mkdir -p "{{BIN_DIR}}"
    @if [ "$(uname)" = "Linux" ]; then mkdir -p "{{LOCAL_BIN}}"; fi

#Downloads the compiled Rust binaries from GitHub
download-core: setup-dirs
    @echo "=> Downloading omp-injector.exe..."
    curl -L -o "{{BIN_DIR}}/omp-injector.exe" "{{REPO_URL}}/omp-injector.exe"
    
    @echo "=> Downloading omp-core library..."
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
    @echo "=> Installing binaries..."
    cp omp-cli "{{BIN_DIR}}/omp-cli"
    
    @echo "=> Generating omp-cli.ini configuration..."
    @if [ "$(uname)" = "Linux" ]; then \
        echo "[Settings]" > "{{BIN_DIR}}/omp-cli.ini"; \
        echo "InjectorPath={{BIN_DIR}}/omp-injector.exe" >> "{{BIN_DIR}}/omp-cli.ini"; \
        echo "IsWine=true" >> "{{BIN_DIR}}/omp-cli.ini"; \
        \
        echo "=> Creating terminal wrapper in {{LOCAL_BIN}}/omp-cli..."; \
        echo '#!/bin/sh' > "{{LOCAL_BIN}}/omp-cli"; \
        echo 'export LD_LIBRARY_PATH="{{BIN_DIR}}:\$LD_LIBRARY_PATH"' >> "{{LOCAL_BIN}}/omp-cli"; \
        echo 'exec "{{BIN_DIR}}/omp-cli" "$@"' >> "{{LOCAL_BIN}}/omp-cli"; \
        chmod +x "{{LOCAL_BIN}}/omp-cli"; \
        \
        echo "=> Creating Linux Desktop Shortcut..."; \
        mkdir -p ~/.local/share/applications; \
        echo "[Desktop Entry]\nName=open.mp Launcher\nExec={{LOCAL_BIN}}/omp-cli ui\nTerminal=true\nType=Application\nCategories=Game;Network;" > ~/.local/share/applications/omp-cli.desktop; \
        chmod +x ~/.local/share/applications/omp-cli.desktop; \
        \
        echo "=> Success! You can now launch 'open.mp Launcher' from your application menu."; \
        echo "=> You can also type 'omp-cli' anywhere in your terminal!"; \
    else \
        echo "[Settings]" > "{{BIN_DIR}}/omp-cli.ini"; \
        echo "InjectorPath={{BIN_DIR}}/omp-injector.exe" >> "{{BIN_DIR}}/omp-cli.ini"; \
        echo "IsWine=false" >> "{{BIN_DIR}}/omp-cli.ini"; \
        echo "=> Success! Execute {{BIN_DIR}}/omp-cli ui to play."; \
    fi