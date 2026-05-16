package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

type Drone struct {
	AddressForSector string  `json:"address_for_sector"`
	AddressForDrone  string  `json:"address_for_drone"`
	ID               int     `json:"id"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	IsBusy           bool    `json:"is_busy"`
	IsOn             bool    `json:"is_on"`
}

type Sensor struct {
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsActive   bool    `json:"is_active"`
	IsCritical bool    `json:"is_critical"`
}

type Sector struct {
	ID                       int     `json:"ID"`
	AddressForSectorAndDrone string  `json:"address_for_sector_and_drone"`
	AddressForSensor         string  `json:"address_for_sensor"`
	Left                     float64 `json:"left"`
	Right                    float64 `json:"right"`
	Top                      float64 `json:"top"`
	Bottom                   float64 `json:"bottom"`
}

var (
	sensor  Sensor
	sector  string
	mu      sync.Mutex
	sensors = []string{
		"RadarCosteiro",
	}
)

// == SENSOR

func runSensor() {
	conn, err := net.DialTimeout("tcp", sector, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao conectar com setor: ", err)
		return
	}

	encoder := json.NewEncoder(conn)

	for {
		r := rand.Float64()

		mu.Lock()
		sensor.IsActive = r > 0.5
		sensor.IsCritical = r > 0.7
		currentSensor := sensor
		mu.Unlock()

		fmt.Println("[SENSOR] Enviando leitura para setor:", sector)
		fmt.Println("[SENSOR] Dados:", currentSensor)

		if err := encoder.Encode(currentSensor); err != nil {
			fmt.Println("Erro ao se comunicar com setor: ", err)
			_ = conn.Close()

			for {
				conn, err = net.DialTimeout("tcp", sector, 2*time.Second)
				if err == nil {
					fmt.Println("Reconectado ao setor: ", sector)
					encoder = json.NewEncoder(conn)
					break
				}

				fmt.Println("Tentando reconectar...")
				time.Sleep(2 * time.Second)
			}
		}

		time.Sleep(15 * time.Second)
	}
}

// == LOAD SETOR

func findSector(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir sectors.json: ", err)
		return false
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Sector
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		fmt.Println("Erro ao ler sectors.json: ", err)
		return false
	}

	x := sensor.X
	y := sensor.Y
	var valid = false

	for _, s := range config {
		if x >= s.Left &&
			x <= s.Right &&
			y <= s.Top &&
			y >= s.Bottom {
			valid = true
			sector = s.AddressForSensor

			fmt.Println("[SENSOR] Sensor localizado em X:", x, "Y:", y)
			fmt.Println("[SENSOR] Setor escolhido:", sector)
		}
	}

	if !valid {
		fmt.Println("Nenhum setor encontrado")
	}

	return valid
}

// == REGISTER

func register() bool {
	typeString := os.Args[1]
	xString := os.Args[2]
	yString := os.Args[3]

	validType := false
	for _, s := range sensors {
		if s == typeString {
			validType = true
			break
		}
	}

	if !validType {
		fmt.Println("Tipo de sensor inválido")
		return false
	}

	val, err := strconv.ParseFloat(xString, 64)
	if err != nil {
		fmt.Println("Valor X inválido")
		return false
	}

	x := val

	val, err = strconv.ParseFloat(yString, 64)
	if err != nil {
		fmt.Println("Valor Y inválido")
		return false
	}

	y := val

	sensor = Sensor{
		Type:       typeString,
		X:          x,
		Y:          y,
		IsActive:   false,
		IsCritical: false,
	}

	return true
}

// == SAVE DATA

func sendSensorToInterface(serverAddress string) {
	mu.Lock()
	currentSensor := sensor
	mu.Unlock()

	conn, err := net.DialTimeout("tcp", serverAddress, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao conectar interface:", err)
		return
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(currentSensor); err != nil {
		fmt.Println("Erro ao enviar sensor para interface:", err)
	}
}

// == MAIN

func main() {
	if len(os.Args) < 4 {
		return
	}

	if !register() {
		return
	}

	sendSensorToInterface("localhost:9300")

	sectorsPath := "../data/sectors.json"

	if !findSector(sectorsPath) {
		return
	}

	go runSensor()

	select {}
}
