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
	SectorID   int     `json:"sector_id"`
	ID         int     `json:"origin_id"`
	Status     string  `json:"status"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsCritical bool    `json:"is_critical"`
	Clock      int     `json:"clock"`
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
	Text    string  `json:"text"`
	Request Request `json:"request"`
	Clock   int     `json:"clock"`
	Drone   Drone   `json:"drone"`
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

// == DRONE

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
}

func calculateDistance(d Drone, r Request) float64 {
	return math.Sqrt(
		math.Pow(d.X-r.X, 2) +
			math.Pow(d.Y-r.Y, 2),
	)
}

func fulfillRequest(request Request) {
	fmt.Println("Drone indo atender requisição: ", request.ID)

	steps := 50
	delay := 100 * time.Millisecond

	for i := 1; i <= steps; i++ {
		mu.Lock()

		dx := request.X - drone.X
		dy := request.Y - drone.Y

		drone.X += dx / float64(steps-i+1)
		drone.Y += dy / float64(steps-i+1)

		mu.Unlock()

		time.Sleep(delay)
	}

	mu.Lock()
	drone.X = request.X
	drone.Y = request.Y
	mu.Unlock()

	fmt.Println("Drone chegou ao local da requisição")

	time.Sleep(10 * time.Second)

	fmt.Println("Drone finalizou requisição:", request.ID)
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
			fmt.Println("Erro ao criar conexão com drone:", err)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			fmt.Println("Erro ao enviar mensagem para drone:", d.ID)
			_ = conn.Close()
			continue
		}

		var response Message

		if err = decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber resposta do drone:", d.ID)
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "UPDATED" {
			fmt.Println("Drone atualizou requisição:", d.ID)
		}

		if response.Text == "REMOVED" {
			fmt.Println("Drone removeu requisição:", d.ID)
		}

		_ = conn.Close()
	}
}

func markRequestAsAttending(request Request, attendingDrone Drone) {
	mu.Lock()
	defer mu.Unlock()

	for i := range requests {
		if requests[i].ID == request.ID && requests[i].SectorID == request.SectorID {
			requests[i].Status = "ATTENDING"
			break
		}
	}

	if drone.ID == attendingDrone.ID {
		drone.IsBusy = true
		return
	}

	for i := range drones {
		if drones[i].ID == attendingDrone.ID {
			drones[i].IsBusy = true
			drones[i].X = attendingDrone.X
			drones[i].Y = attendingDrone.Y
			break
		}
	}
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
}

func dispatchRequests() {
	for {
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

				fmt.Println("Drone escolhido: ", closer.ID, " Requsição: ", r.ID, " Setor: ", r.SectorID)

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
				}

				break
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
		fmt.Println("Erro ao receber mensagem do drone: ", err)
		return
	}

	if message.Text == "ATTENDING" {
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
		mu.Lock()
		currentClock := updateClock(message.Clock)
		mu.Unlock()

		removeRequestDone(message.Request, message.Drone)

		_ = encoder.Encode(Message{
			Text:  "REMOVED",
			Clock: currentClock,
		})
	}
}

func listenDrones() {
	_, port, _ := net.SplitHostPort(drone.AddressForDrone)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (drone): ", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor inicializado (drone)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão (drone): ", err)
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
			fmt.Println("Erro ao conectar com setor: ", s.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			fmt.Println("Erro ao enviar mensagem para setor:", s.ID)
			_ = conn.Close()
			continue
		}

		var response Message

		if err = decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber resposta do setor:", s.ID)
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "UPDATED" {
			fmt.Println("Setor atualizou requisição:", s.ID)
		}

		if response.Text == "REMOVED" {
			fmt.Println("Setor removeu requisição:", s.ID)
		}

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
		fmt.Println("Erro ao receber mensagem do setor")
		return
	}

	switch message.Text {
	case "REQUEST":
		request := message.Request

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
			fmt.Println("Erro ao aceitar conexão (setor): ", err)
			continue
		}

		go handleSector(conn)
	}
}

// == LOAD DATA

func loadSectors(path string) error {
	file, err := os.Open(path)
	if err != nil {
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
		d.IsOn = true

		if d.ID == myID {
			drone = d
			continue
		}

		filtered = append(filtered, d)
	}

	drones = filtered

	return nil
}

// == SAVE DATA

func sendDroneToInterface(serverAddress string) {
	mu.Lock()
	currentDrone := drone
	mu.Unlock()

	conn, err := net.DialTimeout("tcp", serverAddress, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao conectar interface:", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	if err := json.NewEncoder(conn).Encode(currentDrone); err != nil {
		fmt.Println("Erro ao enviar drone para interface:", err)
	}
}

func sendDroneLoop(serverAddress string) {
	for {
		sendDroneToInterface(serverAddress)
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

	dronesPath := "../data/drones.json"
	sectorsPath := "../data/sectors.json"

	if loadDrones(dronesPath, id) != nil {
		return
	}
	if loadSectors(sectorsPath) != nil {
		return
	}

	go listenDrones()
	go listenSectors()

	go dispatchRequests()

	go sendDroneLoop("localhost:9100")

	select {}
}
