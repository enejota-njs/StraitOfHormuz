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
	Text    string  `json:"text"`
	Request Request `json:"request"`
	Clock   int     `json:"clock"`
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
	requests  []Request
)

func incrementClock() int {
	clock++
	return clock
}

func updateClock(receivedClock int) int {
	if receivedClock > clock {
		clock = receivedClock
	}

	clock++
	return clock
}

func sortQueue(request Request) {
	for _, r := range requests {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			return
		}
	}

	index := len(requests)

	for i, r := range requests {
		if request.IsCritical && !r.IsCritical {
			index = i
			break
		}

		if !request.IsCritical && r.IsCritical {
			continue
		}

		if request.Clock < r.Clock {
			index = i
			break
		}

		if request.Clock == r.Clock && request.SectorID < r.SectorID {
			index = i
			break
		}

		if request.Clock == r.Clock &&
			request.SectorID == r.SectorID &&
			request.ID < r.ID {
			index = i
			break
		}
	}

	requests = append(requests[:index], append([]Request{request}, requests[index:]...)...)
}

func sendRequest(sensor Sensor) {
	mu.Lock()

	clockValue := incrementClock()
	requestID++

	request := Request{
		SectorID:   sector.ID,
		ID:         requestID,
		Status:     "PENDING",
		X:          sensor.X,
		Y:          sensor.Y,
		IsCritical: sensor.IsCritical,
		Clock:      clockValue,
	}

	sortQueue(request)

	message := Message{
		Text:    "REQUEST",
		Request: request,
		Clock:   clockValue,
	}

	currentSectors := sectors
	currentDrones := drones

	mu.Unlock()

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("Setor indisponível: ID ", s.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			fmt.Println("Erro ao enviar mensagem para setor: ID ", s.ID)
			_ = conn.Close()
			continue
		}

		var response Message

		if err = decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber resposta do setor: ID ", s.ID)
			_ = conn.Close()
			continue
		}

		if response.Text == "QUEDED" {
			_ = conn.Close()
		}
	}

	for _, d := range currentDrones {
		conn, err := net.DialTimeout("tcp", d.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("Drone indisponível: ID ", d.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			fmt.Println("Erro ao enviar mensagem para drone: ID ", d.ID)
			_ = conn.Close()
			continue
		}

		var response Message

		if err = decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber resposta do drone: ID ", d.ID)
			_ = conn.Close()
			continue
		}

		if response.Text == "QUEDED" {
			_ = conn.Close()
		}
	}
}

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
			go sendRequest(sensor)
		}
	}
}

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
}

func monitorSectors() {
	for {
		mu.Lock()
		currentSectors := sectors
		mu.Unlock()

		for _, s := range currentSectors {
			conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
			if err != nil {
				fmt.Println("Setor offline: ", s.ID)
				continue
			}

			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			message := Message{Text: "PING"}

			if encoder.Encode(message) != nil {
				_ = conn.Close()
				fmt.Println("Falha ao enviar PING para setor: ", s.ID)
				continue
			}

			if decoder.Decode(&message) != nil {
				_ = conn.Close()
				fmt.Println("Falha ao receber PONG do setor: ", s.ID)
				continue
			}

			if message.Text != "PONG" {
				_ = conn.Close()
				fmt.Println("Resposta inválida do setor: ", s.ID)
				continue
			}

			_ = conn.Close()
		}

		time.Sleep(5 * time.Second)
	}
}

func handleSector(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if decoder.Decode(&message) != nil {
		return
	}

	switch message.Text {
	case "REQUEST":
		mu.Lock()
		updateClock(message.Clock)
		sortQueue(message.Request)
		mu.Unlock()
		_ = encoder.Encode(Message{
			Text:  "QUEUED",
			Clock: clock,
		})
		
	case "PING":
		message.Text = "PONG"
		_ = encoder.Encode(message)
	}
}

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
}

func loadSectors(path string, myID int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Sector
	if err = json.NewDecoder(file).Decode(&config); err != nil {
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
}

func loadDrones(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Drone
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	drones = config

	return nil
}

func saveSectorState(path string) error {
	var sectorsList []Sector

	file, err := os.Open(path)

	if err == nil {
		defer func() {
			_ = file.Close()
		}()

		_ = json.NewDecoder(file).Decode(&sectorsList)
	}

	exists := false

	for i := range sectorsList {
		if sectorsList[i].ID == sector.ID {
			sectorsList[i] = sector
			exists = true
			break
		}
	}

	if !exists {
		sectorsList = append(sectorsList, sector)
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

	return encoder.Encode(sectorsList)
}

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
	//savePath := "../data/interface_sectors.json"

	if loadSectors(sectorsPath, id) != nil {
		return
	}
	if loadDrones(dronesPath) != nil {
		return
	}

	//_ = saveSectorState(savePath)

	go listenSectors()
	go monitorSectors()

	go listenSensor()

	select {}
}
