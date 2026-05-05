package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
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

type Command struct {
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
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

type Message struct {
	Text string `json:"text"`
}

type Drone struct {
	Address string  `json:"address"`
	ID      int     `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	IsBusy  bool    `json:"is_busy"`
	IsOn    bool    `json:"is_on"`
}

var (
	sector  Sector
	sectors []Sector
	drones  []Drone
	mu      sync.Mutex
)

// == DRONE

func requestDrone(sensor Sensor) {
	mu.Lock()
	currentDrones := make([]Drone, len(drones))
	copy(currentDrones, drones)
	mu.Unlock()

	for _, drone := range currentDrones {
		conn, err := net.DialTimeout("tcp", drone.Address, 2*time.Second)
		if err != nil {
			fmt.Println("Drone indisponível:", drone.Address)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		command := Command{
			Type:       "REQUEST",
			X:          sensor.X,
			Y:          sensor.Y,
			IsCritical: sensor.IsCritical,
		}

		if err := encoder.Encode(command); err != nil {
			fmt.Println("Erro ao enviar comando para drone:", drone.Address)
			_ = conn.Close()
			continue
		}

		var response string

		if err := decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber resposta do drone:", drone.Address)
			_ = conn.Close()
			continue
		}

		if response == "BUSY" {
			fmt.Println("Drone ocupado:", drone.Address)
			_ = conn.Close()
			continue
		}

		if response == "SERVING" {
			fmt.Println("Drone atendendo solicitação:", drone.Address)
		}

		if err := decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber finalização do drone:", drone.Address)
			_ = conn.Close()
			continue
		}

		if response == "FINISHED" {
			fmt.Println("Drone finalizou solicitação:", drone.Address)
			_ = conn.Close()
			return
		}

		_ = conn.Close()
	}

	fmt.Println("Nenhum drone finalizou a solicitação")
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
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (sensor):", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor (sensor) inicializado")

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

func removeSector(address string) {
	mu.Lock()
	defer mu.Unlock()

	var filtered []Sector

	for _, s := range sectors {
		if s.AddressForSector == address {
			continue
		}

		filtered = append(filtered, s)
	}

	sectors = filtered
} // Finalizada

func monitorSectors() {
	for {
		mu.Lock()
		currentSectors := append([]Sector(nil), sectors...)
		mu.Unlock()

		for _, s := range currentSectors {
			address := s.AddressForSector

			conn, err := net.DialTimeout("tcp", address, 2*time.Second)
			if err != nil {
				removeSector(address)
				continue
			}

			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			message := Message{Text: "PING"}

			if encoder.Encode(message) != nil {
				_ = conn.Close()
				removeSector(address)
				continue
			}

			if decoder.Decode(&message) != nil {
				_ = conn.Close()
				removeSector(address)
				continue
			}

			if message.Text != "PONG" {
				_ = conn.Close()
				removeSector(address)
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
	listener, err := net.Listen("tcp", ":7000")
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (setor):", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor (setor) inicializado na porta 7000")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar com setor:", err)
			continue
		}

		go handleSector(conn)
	}
} // Finalizada

// == LOAD DATA

func getIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
} // Finalizada

func loadSectors(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Sector
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	myIP := getIP()

	var filtered []Sector

	for _, s := range config {
		host, _, err := net.SplitHostPort(s.AddressForSector)
		if err != nil {
			continue
		}

		if host == myIP {
			continue
		}

		filtered = append(filtered, s)
	}

	mu.Lock()
	sectors = filtered
	mu.Unlock()

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

	mu.Lock()
	drones = config
	mu.Unlock()

	return nil
} // Finalizada

// == MAIN

func main() {
	sectorsPath := "../data/sectors.json"
	dronesPath := "../data/drones.json"

	if loadSectors(sectorsPath) != nil {
		return
	}

	if loadDrones(dronesPath) != nil {
		return
	}

	go listenSectors()
	go monitorSectors()

	go listenSensor()

	select {}
}
