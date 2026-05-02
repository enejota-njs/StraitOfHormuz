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
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsActive   bool    `json:"is_active"`
	IsCritical bool    `json:"is_critical"`
}

var (
	sensor  Sensor
	sensors = []string{
		"Radar costeiro",
	}
)

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

func communication(conn net.Conn, mu *sync.Mutex, isActiveSensor, isCriticalSensor bool) {
	encoder := json.NewEncoder(conn)

	for {
		mu.Lock()
		sensor.IsActive = isActiveSensor
		sensor.IsCritical = isCriticalSensor
		currentSensor := sensor
		mu.Unlock()

		if sendSensor(encoder, currentSensor) != nil {
			conn.Close()

			for {
				conn, err := net.DialTimeout("tcp", "localhost:9000", 2*time.Second)
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

func register() {
	reader := bufio.NewReader(os.Stdin)

	var typeString string
	var x, y float64

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
		fmt.Print("Digite a posição X do sensor: ")
		xStr, _ := reader.ReadString('\n')
		xStr = strings.TrimSpace(xStr)

		val, err := strconv.ParseFloat(xStr, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			fmt.Println("Pressione ENTER...")
			reader.ReadString('\n')
			continue
		}

		if val < 0 || val > 100 {
			fmt.Println("X deve estar entre 0 e 100")
			fmt.Println("Pressione ENTER...")
			reader.ReadString('\n')
			continue
		}

		x = val
		break
	}

	for {
		clearTerminal()
		fmt.Print("Digite a posição Y do sensor: ")
		yStr, _ := reader.ReadString('\n')
		yStr = strings.TrimSpace(yStr)

		val, err := strconv.ParseFloat(yStr, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			fmt.Println("Pressione ENTER...")
			reader.ReadString('\n')
			continue
		}

		if val < 0 || val > 100 {
			fmt.Println("Y deve estar entre 0 e 100")
			fmt.Println("Pressione ENTER...")
			reader.ReadString('\n')
			continue
		}

		y = val
		break
	}

	sensor = Sensor{
		Type: typeString,
		X:    x,
		Y:    y,
	}
}

func main() {
	clearTerminal()

	register()

	conn, err := net.DialTimeout("tcp", "localhost:9000", 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao conectar com broker: ", err)
		return
	}

	var mu sync.Mutex
	var isActiveSensor, isCriticalSensor bool

	go monitoring(&isActiveSensor, &isCriticalSensor, &mu)
	go communication(conn, &mu, isActiveSensor, isCriticalSensor)

	select {}
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
