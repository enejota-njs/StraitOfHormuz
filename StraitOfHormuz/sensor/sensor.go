package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
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

type Sector struct {
	AddressForSector string  `json:"address_for_sector"`
	AddressForSensor string  `json:"address_for_sensor"`
	Left             float64 `json:"left"`
	Right            float64 `json:"right"`
	Top              float64 `json:"top"`
	Bottom           float64 `json:"bottom"`
}

type SectorConfig struct {
	Sectors []Sector `json:"sectors.json"`
}

var (
	sensor Sensor
	mu     sync.Mutex

	sensors = []string{
		"Radar costeiro",
	}
)

// == SENSOR

func runSensor(sectorAddress string) {
	conn, err := net.DialTimeout("tcp", sectorAddress, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao conectar com setor:", err)
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

		fmt.Println(currentSensor)

		if err := encoder.Encode(currentSensor); err != nil {
			fmt.Println("Erro ao se comunicar com setor:", err)
			_ = conn.Close()

			for {
				conn, err = net.DialTimeout("tcp", sectorAddress, 2*time.Second)
				if err == nil {
					fmt.Println("Reconectado ao setor:", sectorAddress)
					encoder = json.NewEncoder(conn)
					break
				}

				fmt.Println("Tentando reconectar...")
				time.Sleep(2 * time.Second)
			}
		}

		time.Sleep(1 * time.Second)
	}
}

// == LOAD SETOR

func findSectorByPosition(path string, x, y float64) string {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir sectors.json.json:", err)
		return ""
	}
	defer func() {
		_ = file.Close()
	}()

	var config SectorConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		fmt.Println("Erro ao ler sectors.json.json:", err)
		return ""
	}

	for _, sector := range config.Sectors {
		if x >= sector.Left &&
			x <= sector.Right &&
			y <= sector.Top &&
			y >= sector.Bottom {
			return sector.AddressForSensor
		}
	}

	return ""
}

// == REGISTER

func register() {
	reader := bufio.NewReader(os.Stdin)

	var typeString string
	var x, y float64

	for {
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
			_, _ = reader.ReadString('\n')
			continue
		}

		break
	}

	for {
		fmt.Print("Digite a posição X do sensor: ")
		xStr, _ := reader.ReadString('\n')
		xStr = strings.TrimSpace(xStr)

		val, err := strconv.ParseFloat(xStr, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			fmt.Println("Pressione ENTER...")
			_, _ = reader.ReadString('\n')
			continue
		}

		x = val
		break
	}

	for {
		fmt.Print("Digite a posição Y do sensor: ")
		yStr, _ := reader.ReadString('\n')
		yStr = strings.TrimSpace(yStr)

		val, err := strconv.ParseFloat(yStr, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			fmt.Println("Pressione ENTER...")
			_, _ = reader.ReadString('\n')
			continue
		}

		y = val
		break
	}

	sensor = Sensor{
		Type:       typeString,
		X:          x,
		Y:          y,
		IsActive:   false,
		IsCritical: false,
	}
}

// == MAIN

func main() {
	sectorsPath := "../data/sectors.json"

	register()

	sectorAddress := findSectorByPosition(sectorsPath, sensor.X, sensor.Y)

	if sectorAddress == "" {
		fmt.Println("Nenhum setor encontrado para essa localização")
		return
	}

	go runSensor(sectorAddress)

	select {}
}
