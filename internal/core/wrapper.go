package core

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../omp-launchlib/target/release -lomp_core
#include <stdlib.h>
#include <stdint.h>

extern char* omp_core_fetch_servers();
extern char* omp_core_query_server(const char* ip, uint16_t port);
extern char* omp_core_query_batch(const char* json_targets);
extern char* omp_core_launch(const char* config_json);
extern char* omp_core_query_clients(const char* ip, uint16_t port);
extern void omp_core_free_string(char* s);
*/
import "C"
import (
	"encoding/json"
	"errors"
	"strings"
	"unsafe"
)

// Raw data from the open.mp Web API
type OpenMpServer struct {
	IP  string `json:"ip"`
	Hn  string `json:"hn"`  // Hostname
	Pc  uint32 `json:"pc"`  // Players
	Pm  uint32 `json:"pm"`  // Max Players
	Gm  string `json:"gm"`  // Gamemode
	La  string `json:"la"`  // Language
	Pa  bool   `json:"pa"`  // Password Protected Status
	Omp bool   `json:"omp"` // Is open.mp
}

// Live UDP Query Data
type ServerInfo struct {
	Target     string `json:"target,omitempty"`
	Hostname   string `json:"hostname"`
	Players    uint32 `json:"players"`
	MaxPlayers uint32 `json:"max_players"`
	Gamemode   string `json:"gamemode"`
	Language   string `json:"language"`
	Password   bool   `json:"password"`
	PingMs     uint32 `json:"ping_ms"`
	Error      string `json:"error,omitempty"`
}

type LaunchConfig struct {
	Host            string  `json:"host"`
	Port            uint16  `json:"port"`
	Name            string  `json:"name"`
	Password        *string `json:"password,omitempty"` // Connect Password
	GamePath        string  `json:"game_path"`
	DllPath         string  `json:"dll_path"`
	OmpDllPath      *string `json:"omp_dll_path,omitempty"`
	IsWine          bool    `json:"is_wine"`
	InjectorExePath string  `json:"injector_exe_path"`
}

type ServerClient struct {
	ID    uint8   `json:"id"`
	Name  string  `json:"name"`
	Score int32   `json:"score"`
	Ping  *uint32 `json:"ping"`
}

func FetchServersAPI() ([]OpenMpServer, error) {
	cResult := C.omp_core_fetch_servers()
	defer C.omp_core_free_string(cResult)

	var servers []OpenMpServer
	if err := json.Unmarshal([]byte(C.GoString(cResult)), &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

func QueryBatch(targets []string) ([]ServerInfo, error) {
	if len(targets) == 0 {
		return []ServerInfo{}, nil
	}

	reqBytes, err := json.Marshal(targets)
	if err != nil {
		return nil, err
	}

	cReq := C.CString(string(reqBytes))
	defer C.free(unsafe.Pointer(cReq))

	cResult := C.omp_core_query_batch(cReq)
	defer C.omp_core_free_string(cResult)

	var info []ServerInfo
	err = json.Unmarshal([]byte(C.GoString(cResult)), &info)
	return info, err
}

func QueryServer(ip string, port uint16) (*ServerInfo, error) {
	cIp := C.CString(ip)
	defer C.free(unsafe.Pointer(cIp))

	cResult := C.omp_core_query_server(cIp, C.uint16_t(port))
	defer C.omp_core_free_string(cResult)

	var info ServerInfo
	if err := json.Unmarshal([]byte(C.GoString(cResult)), &info); err != nil {
		return nil, err
	}
	if info.Error != "" {
		return nil, errors.New(info.Error)
	}
	return &info, nil
}

func LaunchGame(config LaunchConfig) error {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}

	cConfig := C.CString(string(configBytes))
	defer C.free(unsafe.Pointer(cConfig))

	cResult := C.omp_core_launch(cConfig)
	defer C.omp_core_free_string(cResult)

	var result struct {
		Success bool    `json:"success"`
		Error   *string `json:"error,omitempty"`
	}
	if err := json.Unmarshal([]byte(C.GoString(cResult)), &result); err != nil {
		return err
	}
	if !result.Success && result.Error != nil {
		return errors.New(*result.Error)
	}
	return nil
}

func QueryClients(ip string, port uint16) ([]ServerClient, error) {
	cIp := C.CString(ip)
	defer C.free(unsafe.Pointer(cIp))

	cResult := C.omp_core_query_clients(cIp, C.uint16_t(port))
	defer C.omp_core_free_string(cResult)

	resultStr := C.GoString(cResult)
	if strings.Contains(resultStr, "\"error\"") {
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(resultStr), &errResp); err == nil {
			return nil, errors.New(errResp.Error)
		}
		return nil, errors.New("failed to fetch clients")
	}

	var clients []ServerClient
	if err := json.Unmarshal([]byte(resultStr), &clients); err != nil {
		return nil, err
	}
	return clients, nil
}
