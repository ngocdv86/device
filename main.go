package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	GOOS = runtime.GOOS
	PORT = 8699
)

var (
	mainMACAddress string
	serialNumber   string
)

type TokenResponse struct {
	Token       string `json:"token"`
	ExpiredTime int    `json:"expired_time"`
}

func getMainMacAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	//Debug
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.HardwareAddr.String() != "" {
			fmt.Println(iface.Name, iface.HardwareAddr.String(), iface.HardwareAddr[0]&2 == 2)
		}
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.HardwareAddr.String() != "" {
			if GOOS == "darwin" && iface.Name == "en0" {
				mainMACAddress = iface.HardwareAddr.String()
				break
			} else if GOOS == "windows" && iface.Name == "Ethernet" {
				mainMACAddress = iface.HardwareAddr.String()
				break
			} else if GOOS == "linux" && (iface.Name == "eth0" || iface.Name == "enp0s3") {
				mainMACAddress = iface.HardwareAddr.String()
				break
			}
		}
	}

	if mainMACAddress == "" {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp != 0 && iface.HardwareAddr.String() != "" && iface.HardwareAddr[0]&2 != 2 {
				mainMACAddress = iface.HardwareAddr.String()
				break
			}
		}
	}

	if mainMACAddress == "" {
		return "", fmt.Errorf("unable to determine main MAC address")
	}

	return mainMACAddress, nil
}

func getSerialNumber() (string, error) {
	var cmd *exec.Cmd
	var out []byte
	var err error

	switch GOOS {
	case "windows":
		cmd = exec.Command("wmic", "bios", "get", "serialnumber")
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}

		serial := strings.Split(string(out), "\n")[1]
		if len(serial) == 0 {
			return "", errors.New("failed to get serial number")
		}

		return strings.TrimSpace(serial), nil

	case "darwin":
		cmd = exec.Command("system_profiler", "SPHardwareDataType")
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}

		serial := ""
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "Serial Number") {
				serial = strings.TrimSpace(strings.Split(line, ":")[1])
				break
			}
		}

		if len(serial) == 0 {
			return "", errors.New("failed to get serial number")
		}

		return serial, nil

	case "linux":
		cmd = exec.Command("sudo", "dmidecode", "-s", "system-serial-number")
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}

		serial := strings.TrimSpace(string(out))
		if len(serial) == 0 {
			return "", errors.New("failed to get serial number")
		}

		return serial, nil

	default:
		return "", errors.New("unsupported platform")
	}
}

func requestToken() (TokenResponse, error) {
	url := "https://jsonplaceholder.typicode.com/posts"
	requestBody, err := json.Marshal(map[string]any{
		"token":        "token",
		"expired_time": time.Now().Unix() + 3600,
	})
	if err != nil {
		return TokenResponse{}, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()

	var response TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return TokenResponse{}, err
	}

	return response, nil
}

func main() {
	fmt.Printf("Running in localhost:%d\n", PORT)

	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		var err error
		if mainMACAddress == "" {
			mainMACAddress, err = getMainMacAddress()
			if err != nil {
				fmt.Printf("Get main MAC address err: %v\n", err)
			}
		}

		if serialNumber == "" {
			serialNumber, err = getSerialNumber()
			if err != nil {
				fmt.Printf("Get Serial number err: %v\n", err)
			}
		}

		token, err := requestToken()
		if err != nil {
			fmt.Printf("requestToken err: %v\n", err)
		}

		fmt.Fprintf(w, "GOOS: %s\nMAC address: %v\nSerial Number: %s\nTest call api: %+v",
			GOOS,
			mainMACAddress,
			serialNumber,
			token)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil))
}
