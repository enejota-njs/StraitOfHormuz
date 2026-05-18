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

type Sensor struct {
	ID         int     `json:"id"`
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsActive   bool    `json:"is_active"`
	IsCritical bool    `json:"is_critical"`
}

type Sector struct {
	ID               int     `json:"ID"`
	AddressForDrone  string  `json:"address_for_drone"`
	AddressForSector string  `json:"address_for_sector"`
	AddressForSensor string  `json:"address_for_sensor"`
	Left             float64 `json:"left"`
	Right            float64 `json:"right"`
	Top              float64 `json:"top"`
	Bottom           float64 `json:"bottom"`
}

var (
	sensor Sensor
	sector string
	mu     sync.Mutex
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

func register(path string) bool {
	if len(os.Args) < 2 {
		fmt.Println("Informe o ID do sensor")
		return false
	}

	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("ID inválido")
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir sensors.json:", err)
		return false
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Sensor
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		fmt.Println("Erro ao ler sensors.json:", err)
		return false
	}

	for _, s := range config {
		if s.ID == id {
			sensor = Sensor{
				ID:         s.ID,
				Type:       s.Type,
				X:          s.X,
				Y:          s.Y,
				IsActive:   false,
				IsCritical: false,
			}

			fmt.Println("[SENSOR] Sensor carregado:", sensor)
			return true
		}
	}

	fmt.Println("Sensor não encontrado com ID:", id)
	return false
}

// == SAVE DATA

func sendSensorToInterface(path string) {
	mu.Lock()
	currentSensor := sensor
	mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir interface.json:", err)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	var config []struct {
		Sectors string `json:"sectors"`
		Drones  string `json:"drones"`
		Sensors string `json:"sensors"`
	}

	if err = json.NewDecoder(file).Decode(&config); err != nil {
		fmt.Println("Erro ao ler interface.json:", err)
		return
	}

	for {
		conn, err := net.DialTimeout("tcp", config[0].Sensors, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao conectar com a interface: ", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if err = json.NewEncoder(conn).Encode(currentSensor); err != nil {
			fmt.Println("Erro ao enviar sensor para interface:", err)
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}

	fmt.Println("Comunicação com a interface concluída")
}

// == MAIN

func main() {
	if len(os.Args) < 2 {
		return
	}

	sensorsPath := "../data/initialization/sensors.json"
	sectorsPath := "../data/initialization/sectors.json"
	intefacePath := "../data/initialization/interface.json"

	if !register(sensorsPath) {
		return
	}
	if !findSector(sectorsPath) {
		return
	}

	sendSensorToInterface(intefacePath)

	go runSensor()

	select {}
}
