# open.mp Terminal Launcher (`omp-cli`)

A cool, and visually stunning CLI server browser and launcher for SA-MP and open.mp. Powered by bubbletea.

## Features
* **Reactive TUI:** Smooth, cool (I hope), async UI powered by Bubble Tea.
* **Live Ping Analytics:** Visualises live UDP ping fluctuation graphs directly in the terminal.
* **Headless Dependency Management:** Automatically downloads, verifies, and extracts specific SA-MP versions and the `omp-client.dll` strictly when needed. 
* **Security:** Encrypts saved Server Passwords and RCON passwords locally using AES-GCM.
* **Linux/Wine Support:** wraps the native Windows injector in Wine.
* **Legacy Import:** Automatically imports your SA-MP favourites from `USERDATA.DAT`.

## Installation

### Using Just
If you have [`just`](https://just.systems/) installed, you can just automatically fetch the core dependencies, build the CLI, and set up your environment:

```bash
just install
```

### Manual Setup
1. Download the `omp-injector.exe` and `omp_core` shared library from the Rust repository.
2. Build the Go CLI
```bash
go build -o omp-cli ./cmd/omp-cli
```
3. Run the initial setup:
```bash
./omp-cli config setup
```

## Commands
* `omp-cli ui` - Launch the interactive TUI (Default).
* `omp-cli query <ip:port>` - Perform a quick CLI query for a server.
* `omp-cli launch --ip <ip> --port <port> --path <gta_path>` - Headless launch.
* `omp-cli config view` - View current configuration.
