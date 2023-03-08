package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

const (
	GOOS = runtime.GOOS
	PORT = 8699
)

var (
	mainMACAddress string
	deviceToken    DeviceToken
	URL            = "https://dev-online-gateway.ghn.vn/sso-v2/public-api/staff/gen-device-token"
)

type DeviceToken struct {
	Token       string    `json:"device_token"`
	ExpiredTime time.Time `json:"expired_time"`
}

type GenDeviceTokenResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    DeviceToken `json:"data"`
}

func getMainMacAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	//Debug
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.HardwareAddr.String() != "" {
			fmt.Println("[debug]", iface.Name, iface.HardwareAddr.String(), iface.HardwareAddr[0]&2 == 2)
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

func requestDeviceToken(deviceID string) (response DeviceToken, err error) {
	requestBody, err := json.Marshal(map[string]any{
		"device_id": deviceID,
	})
	if err != nil {
		return
	}

	resp, err := http.Post(URL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var rs GenDeviceTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&rs)
	if err != nil {
		return
	}

	return rs.Data, nil
}

func execCommand(name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("Error executing command: %s......\n", err.Error())
		return nil, err
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(os.Stdout, stdout)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(os.Stderr, stderr)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		log.Printf("Error waiting for command execution: %s......\n", err.Error())
		return nil, err
	}

	return cmd, nil
}

func execCommandWithoutLog(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return out, err
}

func setup() error {
	switch GOOS {
	case "darwin":
		// https://web.dev/how-to-use-local-https/
		if _, err := execCommand("brew", "install", "mkcert"); err != nil {
			return err
		}
		if _, err := execCommand("brew", "install", "nss"); err != nil {
			return err
		}
		if _, err := execCommand("mkcert", "-install"); err != nil {
			return err
		}
		if _, err := execCommand("mkcert", "-cert-file", "cert.pem", "-key-file", "key.pem", "localhost"); err != nil {
			return err
		}
	case "windows":
		if out, err := execCommandWithoutLog("powershell.exe", "-Command", "Get-ExecutionPolicy"); err != nil {
			return err
		} else {
			fmt.Printf("Get-ExecutionPolicy: %s\n", string(out))
			if string(out) != "AllSigned" {
				if _, err := execCommand("powershell.exe", "-Command", "Set-ExecutionPolicy AllSigned"); err != nil {
					return err
				}
			}
		}

		if out, err := execCommandWithoutLog("powershell.exe", "-Command", "choco -v"); err != nil {
			if _, err := execCommand("powershell.exe", "-Command", "Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))"); err != nil {
				return err
			}
		} else {
			fmt.Printf("choco -v: %s\n", string(out))
		}

		if out, err := execCommandWithoutLog("powershell.exe", "-Command", "mkcert -version"); err != nil {
			if _, err := execCommand("powershell.exe", "-Command", "choco install mkcert"); err != nil {
				return err
			}
		} else {
			fmt.Printf("mkcert -version: %s\n", string(out))
		}

		if _, err := execCommand("powershell.exe", "-Command", "mkcert install"); err != nil {
			return err
		}

		if _, err := execCommand("powershell.exe", "-Command", "mkcert -cert-file cert.pem -key-file key.pem localhost"); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	fmt.Println("Setup....")
	if err := setup(); err != nil {
		fmt.Printf("Setup error: %v\n", err)
	}
	fmt.Printf("\n\nRunning in https://localhost:%d\n", PORT)

	var err error
	mainMACAddress, err = getMainMacAddress()
	if err != nil {
		fmt.Printf("[error] Cannot get MAC address: %v\n", err)
		fmt.Println("[info] Không thể lấy device_id, liên hệ với IT để xử lí.")
	} else {
		fmt.Println("[info] device_id: ", mainMACAddress)
	}

	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

		var err error
		if deviceToken.Token == "" || deviceToken.ExpiredTime.Before(time.Now()) {
			deviceToken, err = requestDeviceToken(mainMACAddress)
			if err != nil {
				fmt.Printf("Request device token err: %v\n", err)
				return
			}
		}

		jsonBytes, err := json.Marshal(deviceToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(jsonBytes)
	})

	err = http.ListenAndServeTLS(fmt.Sprintf(":%d", PORT), "cert.pem", "key.pem", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
