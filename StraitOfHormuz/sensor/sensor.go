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
		fmt.Println("Erro ao se comunicar com servidor do setor: ", err)
		return
	}

	encoder := json.NewEncoder(conn)

	for {
		r := rand.Float64()

		mu.Lock()
		sensor.IsActive = r > 0.6
		sensor.IsCritical = r > 0.8
		currentSensor := sensor
		mu.Unlock()

		fmt.Printf("\nSensor enviando -> ID: %d | Type: %s | X: %.2f | Y: %.2f | Active: %t | Critical: %t\n", currentSensor.ID, currentSensor.Type, currentSensor.X, currentSensor.Y, currentSensor.IsActive, currentSensor.IsCritical)

		if err := encoder.Encode(currentSensor); err != nil {
			_ = conn.Close()

			for {
				conn, err = net.DialTimeout("tcp", sector, 2*time.Second)
				if err == nil {
					fmt.Println("Erro ao se comunicar com servidor do setor: ", err)

					break
				}

				fmt.Println("Tentando reconectar")
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
		fmt.Println("Erro ao abrir arquivo dos setores: ", err)
		return false
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Sector
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return false
	}

	x := sensor.X
	y := sensor.Y

	for _, s := range config {
		if x >= s.Left &&
			x <= s.Right &&
			y <= s.Top &&
			y >= s.Bottom {
			sector = s.AddressForSensor
			return true
		}
	}

	return false
}

// == REGISTER

func register(path string) bool {
	id, err := strconv.Atoi(os.Args[1])

	if err != nil {
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo dos sensores: ", err)
		return false
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Sensor
	if err := json.NewDecoder(file).Decode(&config); err != nil {
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

			return true
		}
	}

	return false
}

// == SAVE DATA

func sendSensorToInterface(path string) {
	mu.Lock()
	currentSensor := sensor
	mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo da interface: ", err)
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
		return
	}

	for {
		conn, err := net.DialTimeout("tcp", config[0].Sensors, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se comunicar com servidor da interface: ", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if err = json.NewEncoder(conn).Encode(currentSensor); err != nil {
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}
}

// == MAIN

func main() {
	if len(os.Args) < 2 {
		return
	}

	sensorsPath := "data/initialization/sensors.json"
	sectorsPath := "data/initialization/sectors.json"
	intefacePath := "data/initialization/interface.json"

	if !register(sensorsPath) {
		fmt.Println("Erro ao registrar sensor")
		return
	}
	if !findSector(sectorsPath) {
		fmt.Println("Erro ao procurar setor")
		return
	}

	go sendSensorToInterface(intefacePath)

	go runSensor()

	select {}
}
