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
	ID         int     `json:"id"`
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsActive   bool    `json:"is_active"`
	IsCritical bool    `json:"is_critical"`
}

type Request struct {
	SectorID         int     `json:"sector_id"`
	ID               int     `json:"origin_id"`
	Status           string  `json:"status"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	IsCritical       bool    `json:"is_critical"`
	Clock            int     `json:"clock"`
	AttendingDroneID int     `json:"attending_drone_id"`
}

type Sector struct {
	ID               int     `json:"id"`
	AddressForDrone  string  `json:"address_for_drone"`
	AddressForSector string  `json:"address_for_sector"`
	AddressForSensor string  `json:"address_for_sensor"`
	Left             float64 `json:"left"`
	Right            float64 `json:"right"`
	Top              float64 `json:"top"`
	Bottom           float64 `json:"bottom"`
}

type Message struct {
	Text     string    `json:"text"`
	Requests []Request `json:"requests"`
	Request  Request   `json:"request"`
	Clock    int       `json:"clock"`
	Drone    Drone     `json:"drone"`
}

type Drone struct {
	ID               int     `json:"id"`
	AddressForSector string  `json:"address_for_sector"`
	AddressForDrone  string  `json:"address_for_drone"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	IsBusy           bool    `json:"is_busy"`
	IsOn             bool    `json:"is_on"`
}

var (
	mu        sync.Mutex
	sector    Sector
	clock     int
	requestID int
	sectors   []Sector
	drones    []Drone
	requests  []Request
)

// == CLOCK

func incrementClock() int {
	clock++
	return clock
}

func updateClock(receivedClock int) int {
	if receivedClock > clock {
		clock = receivedClock
	}

	incrementClock()

	return clock
}

// == REQUEST

func addRequestToQueue(request Request) {
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

	fmt.Println("\nFila atual:\n")
	for i, r := range requests {
		fmt.Println(" ", i, "->",
			"Sector:", r.SectorID,
			"ID:", r.ID,
			"Status:", r.Status,
			"Critical:", r.IsCritical,
			"Clock:", r.Clock,
		)
	}
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

	fmt.Printf("\nNova requisição criada -> SectorID: %d | RequestID: %d | X: %.2f | Y: %.2f | Critical: %t | Clock: %d\n", request.SectorID, request.ID, request.X, request.Y, request.IsCritical, request.Clock)

	addRequestToQueue(request)

	go sendRequestToInterface("../data/initialization/interface.json", request)

	message := Message{
		Text:    "REQUEST",
		Request: request,
		Clock:   clockValue,
	}

	currentSectors := append([]Sector(nil), sectors...)
	currentDrones := append([]Drone(nil), drones...)

	mu.Unlock()

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("\nSetor indisponível: ID ", s.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message

		if err = decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "QUEUED" {
			_ = conn.Close()
		}
	}

	for _, d := range currentDrones {
		conn, err := net.DialTimeout("tcp", d.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("\nDrone indisponível: ID ", d.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message

		if err = decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "QUEUED" {
			_ = conn.Close()
		}
	}
}

// == DRONE

func markRequestAsAttending(request Request, attendingDrone Drone) {
	fmt.Printf("\nDrone aceitou requisição -> DroneID: %d | SectorID: %d | RequestID: %d\n", attendingDrone.ID, request.SectorID, request.ID)

	for i := range requests {
		if requests[i].SectorID == request.SectorID && requests[i].ID == request.ID {
			requests[i].Status = "ATTENDING"
			requests[i].AttendingDroneID = attendingDrone.ID
			break
		}
	}
}

func removeRequestDone(request Request) {
	fmt.Printf("\nRequisição finalizada -> SectorID: %d | RequestID: %d\n", request.SectorID, request.ID)

	var filtered []Request

	for _, r := range requests {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			continue
		}

		filtered = append(filtered, r)
	}

	requests = filtered
}

func handleDrone(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if decoder.Decode(&message) != nil {
		return
	}

	switch message.Text {
	case "ATTENDING":
		mu.Lock()
		currentClock := updateClock(message.Clock)
		markRequestAsAttending(message.Request, message.Drone)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})

	case "DONE":
		mu.Lock()
		currentClock := updateClock(message.Clock)
		removeRequestDone(message.Request)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "REMOVED",
			Clock: currentClock,
		})

	case "SYNC_REQUESTS":
		mu.Lock()
		currentClock := updateClock(message.Clock)
		currentRequests := append([]Request(nil), requests...)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:     "REQUESTS_SYNCED",
			Requests: currentRequests,
			Clock:    currentClock,
		})
	}
}

func listenDrone() {
	_, port, _ := net.SplitHostPort(sector.AddressForDrone)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos drones: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (drone)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleDrone(conn)
	}
}

// == SENSOR

func handleSensor(conn net.Conn) {
	decoder := json.NewDecoder(conn)

	var sensor Sensor

	for {
		if err := decoder.Decode(&sensor); err != nil {
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
		fmt.Println("Erro ao iniciar porta dos sensores: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (sensor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleSensor(conn)
	}
}

// == SECTOR

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
		fmt.Printf("\nRequisição recebida -> SectorID: %d | RequestID: %d | X: %.2f | Y: %.2f | Critical: %t | Clock: %d\n", message.Request.SectorID, message.Request.ID, message.Request.X, message.Request.Y, message.Request.IsCritical, message.Request.Clock)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		addRequestToQueue(message.Request)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "QUEUED",
			Clock: currentClock,
		})
	}
}

func listenSectors() {
	_, port, _ := net.SplitHostPort(sector.AddressForSector)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos setores: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (setor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleSector(conn)
	}
}

// == LOAD DATA

func loadSectors(path string, myID int) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de setores: ", err)
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
		fmt.Println("Erro ao abrir arquivo de drones: ", err)
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

// == SAVE DATA

func sendRequestToInterface(path string, request Request) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de interface: ", err)
		return
	}
	defer file.Close()

	var config []struct {
		Sectors  string `json:"sectors"`
		Drones   string `json:"drones"`
		Sensors  string `json:"sensors"`
		Requests string `json:"requests"`
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return
	}

	conn, err := net.DialTimeout("tcp", config[0].Requests, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao conectar com interface para enviar requisição: ", err)
		return
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return
	}
}

func sendSectorToInterface(path string) {
	mu.Lock()
	currentSector := sector
	mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de interface: ", err)
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
		conn, err := net.DialTimeout("tcp", config[0].Sectors, 2*time.Second)
		if err != nil {
			fmt.Println("Interface indisponível, tentando novamente...")
			time.Sleep(1 * time.Second)
			continue
		}

		if err = json.NewEncoder(conn).Encode(currentSector); err != nil {
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

	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return
	}

	sectorsPath := "../data/initialization/sectors.json"
	dronesPath := "../data/initialization/drones.json"
	intefacePath := "../data/initialization/interface.json"

	if loadSectors(sectorsPath, id) != nil {
		return
	}
	if loadDrones(dronesPath) != nil {
		return
	}

	go sendSectorToInterface(intefacePath)

	go listenSensor()
	go listenSectors()
	go listenDrone()

	select {}
}
