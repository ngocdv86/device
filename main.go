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

var (
	GOOS = runtime.GOOS
)

type Response struct {
	ID          int    `json:"id"`
	Token       string `json:"token"`
	ExpiredTime int    `json:"expired_time"`
}

func getMainMacAddress() (string, error) {
	var mainMacAddress string
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.HardwareAddr.String() != "" {
			if GOOS == "darwin" && iface.Name == "en0" {
				mainMacAddress = iface.HardwareAddr.String()
				break
			} else if GOOS == "windows" && iface.Name == "Ethernet" {
				mainMacAddress = iface.HardwareAddr.String()
				break
			} else if GOOS == "linux" && (iface.Name == "eth0" || iface.Name == "enp0s3") {
				mainMacAddress = iface.HardwareAddr.String()
				break
			}
		}
	}
	if mainMacAddress == "" {
		return "", fmt.Errorf("unable to determine main MAC address")
	}
	return mainMacAddress, nil
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

func getToken() (Response, error) {
	url := "https://jsonplaceholder.typicode.com/posts"
	requestBody, err := json.Marshal(map[string]any{
		"token":        "token",
		"expired_time": time.Now().Unix() + 3600,
	})
	if err != nil {
		return Response{}, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return Response{}, err
	}

	return response, nil
}

func main() {
	fmt.Println("Running...")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mainMacAddress, err := getMainMacAddress()
		if err != nil {
			log.Fatal(err)
		}

		serialNumber, err := getSerialNumber()
		if err != nil {
			log.Fatal(err)
		}

		rs, err := getToken()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Fprintf(w, "MAC address: %v\nSerial Number: %s\nTest call api: %+v", mainMacAddress, serialNumber, rs)
	})

	log.Fatal(http.ListenAndServe(":8081", nil))
}
