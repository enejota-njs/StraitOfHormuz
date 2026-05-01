package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Sensor struct {
	Type       string `json:"type"`
	Longitude  int    `json:"longitude"`
	IsActive   bool   `json:"is_active"`
	IsCritical bool   `json:"is_critical"`
}

var sensors = []string{
	"Radar costeiro",
}

func sendSensor(encoder *json.Encoder, sensor Sensor) error {
	err := encoder.Encode(sensor)

	if err != nil {
		fmt.Println("Erro ao enviar dados do sensor ", err)
		return err
	}

	return nil
}

func monitoring(isActive, isCritical *bool, mu *sync.Mutex) {
	for {
		r := rand.Float64()
		mu.Lock()

		*isActive = r > 0.5
		*isCritical = r > 0.7

		mu.Unlock()

		time.Sleep(1 * time.Second)
	}
}

func register() (string, int) {
	reader := bufio.NewReader(os.Stdin)

	var typeString string
	var longitude int

	for {
		clearTerminal()
		fmt.Print("Digite o tipo do sensor: ")
		typeString, _ = reader.ReadString('\n')
		typeString = strings.TrimSpace(typeString)

		validType := false
		for _, s := range sensors {
			if s == typeString {
				validType = true
				break
			}
		}

		if !validType {
			fmt.Println("Tipo de sensor inválido")
			fmt.Println("Pressione ENTER...")
			reader.ReadString('\n')
			continue
		}

		break
	}

	for {
		clearTerminal()
		fmt.Print("Digite a longitude do sensor: ")
		longitudeString, _ := reader.ReadString('\n')
		longitudeString = strings.TrimSpace(longitudeString)

		var err error
		longitude, err = strconv.Atoi(longitudeString)
		if err != nil {
			fmt.Println("Longitude inválida")
			fmt.Println("Pressione ENTER...")
			reader.ReadString('\n')
			continue
		}

		if longitude < 0 || longitude > 100 {
			fmt.Println("Longitude inválida")
			fmt.Println("Pressione ENTER...")
			reader.ReadString('\n')
			continue
		}

		break
	}

	return typeString, longitude
}

func main() {
	clearTerminal()
	address := "localhost:7001"

	typeSensor, longitudeSensor := register()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("Erro ao conectar com broker: ", err)
		return
	}

	var mu sync.Mutex
	var isActiveSensor, isCriticalSensor bool

	go monitoring(&isActiveSensor, &isCriticalSensor, &mu)

	encoder := json.NewEncoder(conn)

	for {
		mu.Lock()

		active := isActiveSensor
		critical := isCriticalSensor

		mu.Unlock()

		sensor := Sensor{
			Type:       typeSensor,
			Longitude:  longitudeSensor,
			IsActive:   active,
			IsCritical: critical,
		}

		if sendSensor(encoder, sensor) != nil {
			conn.Close()

			for {
				conn, err = net.Dial("tcp", address)
				if err == nil {
					encoder = json.NewEncoder(conn)
					break
				}
				time.Sleep(2 * time.Second)
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func clearTerminal() {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}
