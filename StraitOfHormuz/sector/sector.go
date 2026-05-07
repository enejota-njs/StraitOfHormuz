package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

type State struct {
	Drones  []Drone  `json:"drones"`
	Sensors []Sensor `json:"sensors"`
	Sectors []Sector `json:"sectors"`
}

type Sensor struct {
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsActive   bool    `json:"is_active"`
	IsCritical bool    `json:"is_critical"`
}

type Request struct {
	SectorID   int     `json:"sector_id"`
	ID         int     `json:"origin_id"`
	Status     string  `json:"status"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsCritical bool    `json:"is_critical"`
	Clock      int     `json:"clock"`
}

type Sector struct {
	ID               int     `json:"ID"`
	AddressForSector string  `json:"address_for_sector"`
	AddressForSensor string  `json:"address_for_sensor"`
	Left             float64 `json:"left"`
	Right            float64 `json:"right"`
	Top              float64 `json:"top"`
	Bottom           float64 `json:"bottom"`
}

type Message struct {
	Text string `json:"text"`
}

type Drone struct {
	AddressForSector string  `json:"address_for_sector"`
	AddressForDrone  string  `json:"address_for_drone"`
	ID               int     `json:"id"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	IsBusy           bool    `json:"is_busy"`
	IsOn             bool    `json:"is_on"`
}

var (
	clock     int
	sectors   []Sector
	requestID int
	drones    []Drone
	mu        sync.Mutex
	sector    Sector
)

// == DRONE

func requestDrone(sensor Sensor) {
	mu.Lock()

	clock++
	requestID++

	request := Request{
		SectorID:   sector.ID,
		ID:         requestID,
		Status:     "PENDING",
		X:          sensor.X,
		Y:          sensor.Y,
		IsCritical: sensor.IsCritical,
		Clock:      clock,
	}

	fmt.Println("[SETOR", sector.ID, "] Nova requisição criada")
	fmt.Println("ID:", request.ID)
	fmt.Println("Clock:", request.Clock)
	fmt.Println("X:", request.X, "Y:", request.Y)
	fmt.Println("Crítica:", request.IsCritical)

	currentDrones := append([]Drone(nil), drones...)

	mu.Unlock()

	for _, d := range currentDrones {
		conn, err := net.DialTimeout("tcp", d.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("Drone indisponível: ID ", d.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		fmt.Println("[SETOR", sector.ID, "] Enviando requisição", request.ID, "para Drone", d.ID)

		if err := encoder.Encode(request); err != nil {
			fmt.Println("Erro ao enviar requisição para drone: ID ", d.ID)
			_ = conn.Close()
			continue
		}

		var response Message

		if err := decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber resposta do drone: ID ", d.ID)
			_ = conn.Close()
			continue
		}

		fmt.Println("[SETOR", sector.ID, "] Drone", d.ID, "respondeu:", response.Text)

		if response.Text == "INVALID_COMMAND" {
			fmt.Println("Requisição inválida")
		}

		if response.Text == "QUEUED" {
			fmt.Println("[SETOR", sector.ID, "] Drone", d.ID, "recebeu requisição", request.SectorID, "-", request.ID)
		}

		_ = conn.Close()
	}
}

// == SENSOR

func handleSensor(conn net.Conn) {
	decoder := json.NewDecoder(conn)
	var sensor Sensor

	for {
		if err := decoder.Decode(&sensor); err != nil {
			fmt.Println("Erro ao receber sensor: ", err)
			_ = conn.Close()
			return
		}

		if sensor.IsActive {
			go requestDrone(sensor)
		}
	}
} // Finalizada

func listenSensor() {
	_, port, _ := net.SplitHostPort(sector.AddressForSensor)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (sensor): ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (sensor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar com sensor: ", err)
			continue
		}

		go handleSensor(conn)
	}
} // Finalizada

// == SECTOR

func monitorSectors() {
	for {
		mu.Lock()
		currentSectors := append([]Sector(nil), sectors...)
		mu.Unlock()

		for _, s := range currentSectors {
			address := s.AddressForSector

			conn, err := net.DialTimeout("tcp", address, 2*time.Second)
			if err != nil {
				fmt.Println("Setor offline: ", address)
				continue
			}

			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			message := Message{Text: "PING"}

			if encoder.Encode(message) != nil {
				_ = conn.Close()
				fmt.Println("Falha ao enviar PING para setor:", address)
				continue
			}

			if decoder.Decode(&message) != nil {
				_ = conn.Close()
				fmt.Println("Falha ao receber PONG do setor:", address)
				continue
			}

			if message.Text != "PONG" {
				_ = conn.Close()
				fmt.Println("Resposta inválida do setor:", address)
				continue
			}

			_ = conn.Close()
		}

		time.Sleep(5 * time.Second)
	}
} // Finalizada

func handleSector(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if decoder.Decode(&message) != nil {
		return
	}

	if message.Text == "PING" {
		message.Text = "PONG"
		_ = encoder.Encode(message)
	}
} // Finalizada

func listenSectors() {
	_, port, _ := net.SplitHostPort(sector.AddressForSector)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (setor): ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (setor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar com setor: ", err)
			continue
		}

		go handleSector(conn)
	}
} // Finalizada

// == LOAD DATA

func loadSectors(path string, myID int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Sector
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	var filtered []Sector

	for _, s := range config {
		if s.ID == myID {
			sector = s
			continue
		}

		filtered = append(filtered, s)
	}

	sectors = filtered

	return nil
} // Finalizada

func loadDrones(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Drone
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	drones = config

	return nil
} // Finalizada

// == SAVE DATA

func saveSectorState(path string) error {
	file, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	state := State{}

	if err == nil {
		defer func() {
			_ = file.Close()
		}()

		_ = json.NewDecoder(file).Decode(&state)
	}

	exists := false

	for i := range state.Sectors {
		if state.Sectors[i].ID == sector.ID {
			state.Sectors[i] = sector
			exists = true
			break
		}
	}

	if !exists {
		state.Sectors = append(state.Sectors, sector)
	}

	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = output.Close()
	}()

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	return encoder.Encode(state)
}

// == MAIN

func main() {
	if len(os.Args) < 2 {
		return
	}

	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return
	}

	sectorsPath := "../data/sectors.json"
	dronesPath := "../data/drones.json"
	savePath := "../data/world.json"

	if loadSectors(sectorsPath, id) != nil {
		return
	}

	_ = saveSectorState(savePath)

	if loadDrones(dronesPath) != nil {
		return
	}

	go listenSectors()
	go monitorSectors()

	go listenSensor()

	select {}
}
