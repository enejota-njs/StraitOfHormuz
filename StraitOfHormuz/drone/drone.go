package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

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

type Drone struct {
	AddressForSector string  `json:"address_for_sector"`
	AddressForDrone  string  `json:"address_for_drone"`
	ID               int     `json:"id"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	IsBusy           bool    `json:"is_busy"`
	IsOn             bool    `json:"is_on"`
}

type Message struct {
	Text     string    `json:"text"`
	Requests []Request `json:"requests"`
	Request  Request   `json:"request"`
	Clock    int       `json:"clock"`
	Drone    Drone     `json:"drone"`
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
	drone    Drone
	mu       sync.Mutex
	drones   []Drone
	requests []Request
	sectors  []Sector
	clock    int
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

func syncRequestsFromSectors() {
	mu.Lock()
	clockValue := incrementClock()
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	message := Message{
		Text:  "SYNC_REQUESTS",
		Clock: clockValue,
	}

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForDrone, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se conectar com setor: ", err)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err := encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message
		if err := decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)

		for _, r := range response.Requests {
			addRequestToQueue(r)
		}

		mu.Unlock()

		_ = conn.Close()
	}
}

// == DRONE

func handleDroneCrash(crashedDroneID int) {
	mu.Lock()
	defer mu.Unlock()

	droneCrashed := false
	var deadDrone Drone

	for i := range drones {
		if drones[i].ID == crashedDroneID && drones[i].IsOn {
			drones[i].IsOn = false
			drones[i].IsBusy = false
			droneCrashed = true
			deadDrone = drones[i]
			break
		}
	}

	if !droneCrashed {
		return
	}

	go sendDeadDroneToInterface("../data/initialization/interface.json", deadDrone)

	for i := range requests {
		if requests[i].Status == "ATTENDING" && requests[i].AttendingDroneID == crashedDroneID {
			requests[i].Status = "PENDING"
			requests[i].AttendingDroneID = 0

			pendingRequest := requests[i]

			go sendRequestToInterface(
				"../data/initialization/interface.json",
				pendingRequest,
			)

			go warnDrones("PENDING", pendingRequest)
			go warnSectors("PENDING", pendingRequest)
		}
	}
}

func monitorDrones() {
	for {
		mu.Lock()
		currentDrones := append([]Drone(nil), drones...)
		mu.Unlock()

		for _, d := range currentDrones {
			if d.IsOn && d.ID != drone.ID {
				conn, err := net.DialTimeout("tcp", d.AddressForDrone, 2*time.Second)
				if err != nil {
					fmt.Println("Drone não respondeu: ", d.ID)
					handleDroneCrash(d.ID)
				} else {
					_ = conn.Close()
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
}

func calculateDistance(d Drone, r Request) float64 {
	distance := math.Sqrt(
		math.Pow(d.X-r.X, 2) +
			math.Pow(d.Y-r.Y, 2),
	)

	return distance
}

func fulfillRequest(request Request) {
	speed := 5.0
	delay := 100 * time.Millisecond

	for {
		mu.Lock()
		dx := request.X - drone.X
		dy := request.Y - drone.Y
		distance := math.Sqrt(dx*dx + dy*dy)

		if distance <= speed {
			drone.X = request.X
			drone.Y = request.Y
			mu.Unlock()
			break
		}

		drone.X += (dx / distance) * speed
		drone.Y += (dy / distance) * speed
		mu.Unlock()

		time.Sleep(delay)
	}

	time.Sleep(10 * time.Second)
}

func warnDrones(text string, request Request) {
	mu.Lock()
	clockValue := incrementClock()
	currentDrones := append([]Drone(nil), drones...)
	currentDrone := drone
	mu.Unlock()

	message := Message{
		Text:    text,
		Request: request,
		Drone:   currentDrone,
		Clock:   clockValue,
	}

	for _, d := range currentDrones {
		conn, err := net.DialTimeout("tcp", d.AddressForDrone, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se comunicar com Drone ID: ", d.ID)
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

		_ = conn.Close()
	}
}

func markRequestAsPending(request Request) {
	mu.Lock()
	defer mu.Unlock()

	for i := range requests {
		if requests[i].ID == request.ID && requests[i].SectorID == request.SectorID {
			requests[i].Status = "PENDING"
			requests[i].AttendingDroneID = 0
			break
		}
	}

	request.Status = "PENDING"
	request.AttendingDroneID = 0

	go sendRequestToInterface("../data/initialization/interface.json", request)
}

func markRequestAsAttending(request Request, attendingDrone Drone) {
	mu.Lock()
	defer mu.Unlock()

	for i := range requests {
		if requests[i].ID == request.ID && requests[i].SectorID == request.SectorID {
			requests[i].Status = "ATTENDING"
			requests[i].AttendingDroneID = attendingDrone.ID
			break
		}
	}

	if drone.ID == attendingDrone.ID {
		drone.IsBusy = true
	} else {
		for i := range drones {
			if drones[i].ID == attendingDrone.ID {
				drones[i].IsBusy = true
				drones[i].IsOn = true
				drones[i].X = attendingDrone.X
				drones[i].Y = attendingDrone.Y
				break
			}
		}
	}

	request.Status = "ATTENDING"
	go sendRequestToInterface("../data/initialization/interface.json", request)
}

func removeRequestDone(request Request, finishedDrone Drone) {
	mu.Lock()
	defer mu.Unlock()

	var filtered []Request

	for _, r := range requests {
		if r.ID == request.ID && r.SectorID == request.SectorID {
			continue
		}

		filtered = append(filtered, r)
	}

	requests = filtered

	if drone.ID == finishedDrone.ID {
		drone.IsBusy = false
		drone.X = finishedDrone.X
		drone.Y = finishedDrone.Y
	}

	for i := range drones {
		if drones[i].ID == finishedDrone.ID {
			drones[i].IsBusy = false
			drones[i].X = finishedDrone.X
			drones[i].Y = finishedDrone.Y

			break
		}
	}

	request.Status = "DONE"
	go sendRequestToInterface("../data/initialization/interface.json", request)
}

func dispatchRequests() {
	for {
		syncRequestsFromSectors()

		mu.Lock()

		currentDrone := drone
		currentRequests := append([]Request(nil), requests...)
		currentDrones := append([]Drone(nil), drones...)

		mu.Unlock()

		if currentDrone.IsOn && !currentDrone.IsBusy {
			for _, r := range currentRequests {
				if r.Status != "PENDING" {
					continue
				}

				closer := currentDrone
				distance := calculateDistance(currentDrone, r)

				for _, d := range currentDrones {
					if !d.IsOn || d.IsBusy {
						continue
					}

					tempDistance := calculateDistance(d, r)

					if tempDistance < distance {
						distance = tempDistance
						closer = d
					}

					if tempDistance == distance && d.ID < closer.ID {
						closer = d
					}
				}

				fmt.Println(
					"\nDrone escolhido",
					"| Drone: ", closer.ID,
					"| Request: ", r.ID,
					"| Sector: ", r.SectorID,
				)

				if closer.ID == currentDrone.ID {
					markRequestAsAttending(r, currentDrone)

					warnDrones("ATTENDING", r)
					warnSectors("ATTENDING", r)

					fulfillRequest(r)

					mu.Lock()
					updatedDrone := drone
					mu.Unlock()

					removeRequestDone(r, updatedDrone)

					warnDrones("DONE", r)
					warnSectors("DONE", r)

					break
				}
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func handleDrones(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message
	if err := decoder.Decode(&message); err != nil {
		return
	}

	if message.Text == "ATTENDING" {
		fmt.Printf("\nAviso de ATTENDING recebido -> DroneID: %d | SectorID: %d | RequestID: %d\n", message.Drone.ID, message.Request.SectorID, message.Request.ID)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		mu.Unlock()

		markRequestAsAttending(message.Request, message.Drone)

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})
	}

	if message.Text == "DONE" {
		fmt.Printf("\nAviso de DONE recebido -> DroneID: %d | SectorID: %d | RequestID: %d\n", message.Drone.ID, message.Request.SectorID, message.Request.ID)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		mu.Unlock()

		removeRequestDone(message.Request, message.Drone)

		_ = encoder.Encode(Message{
			Text:  "REMOVED",
			Clock: currentClock,
		})
	}

	if message.Text == "PENDING" {
		fmt.Printf("\nAviso de PENDING recebido -> SectorID: %d | RequestID: %d\n", message.Request.SectorID, message.Request.ID)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		mu.Unlock()

		markRequestAsPending(message.Request)

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})
	}
}

func listenDrones() {
	_, port, _ := net.SplitHostPort(drone.AddressForDrone)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos drones: ", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor inicializado (drone)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleDrones(conn)
	}
}

// == SECTOR

func warnSectors(text string, request Request) {
	mu.Lock()
	clockValue := incrementClock()
	currentDrone := drone
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	message := Message{
		Text:    text,
		Request: request,
		Drone:   currentDrone,
		Clock:   clockValue,
	}

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForDrone, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se comunicar com Setor ID: ", s.ID)
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

		_ = conn.Close()
	}
}

func handleSector(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if err := decoder.Decode(&message); err != nil {
		return
	}

	switch message.Text {
	case "REQUEST":
		request := message.Request

		fmt.Printf("\nRequisição recebida -> SectorID: %d | RequestID: %d | X: %.2f | Y: %.2f | Critical: %t | Clock: %d\n", request.SectorID, request.ID, request.X, request.Y, request.IsCritical, request.Clock)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		addRequestToQueue(request)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "QUEUED",
			Clock: currentClock,
		})
	}
}

func listenSectors() {
	_, port, _ := net.SplitHostPort(drone.AddressForSector)

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

func loadSectors(path string) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo dos setores: ", err)
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Sector
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	sectors = config

	return nil
}

func loadDrones(path string, myID int) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo dos drones: ", err)
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Drone
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	var filtered []Drone
	for _, d := range config {
		d.X = 0
		d.Y = 0
		d.IsBusy = false

		if d.ID == myID {
			d.IsOn = true
			drone = d
			continue
		}

		d.IsOn = false
		filtered = append(filtered, d)
	}

	drones = filtered

	return nil
}

// == SAVE DATA

func sendDroneToInterface(path string) {
	mu.Lock()
	currentDrone := drone
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
		conn, err := net.DialTimeout("tcp", config[0].Drones, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se conectar com servidor da interface: ", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if err := json.NewEncoder(conn).Encode(currentDrone); err != nil {
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}
}

func sendRequestToInterface(path string, request Request) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo da interface: ", err)
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
		fmt.Println("Erro ao se conectar com servidor da interface: ", err)
		return
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return
	}
}

func sendDeadDroneToInterface(path string, deadDrone Drone) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo da interface: ", err)
		return
	}
	defer file.Close()

	var config []struct {
		Drones string `json:"drones"`
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return
	}

	conn, err := net.DialTimeout("tcp", config[0].Drones, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao se comunicar com servidor da interface: ", err)
		return
	}
	defer conn.Close()

	_ = json.NewEncoder(conn).Encode(deadDrone)
}

func sendDroneLoop(path string) {
	for {
		sendDroneToInterface(path)
		time.Sleep(500 * time.Millisecond)
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

	dronesPath := "../data/initialization/drones.json"
	sectorsPath := "../data/initialization/sectors.json"
	intefacePath := "../data/initialization/interface.json"

	if loadDrones(dronesPath, id) != nil {
		return
	}
	if loadSectors(sectorsPath) != nil {
		return
	}

	go listenDrones()
	go listenSectors()
	go dispatchRequests()
	go monitorDrones()
	go sendDroneLoop(intefacePath)

	select {}
}
