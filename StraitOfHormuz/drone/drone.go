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

type Notice struct {
	Text    string  `json:"text"`
	Request Request `json:"request"`
	Drone   Drone   `json:"drone"`
}

type Message struct {
	Text string `json:"text"`
}

var (
	clock    int
	drone    Drone
	mu       sync.Mutex
	drones   []Drone
	requests []Request
)

// == DRONE

func calculateDistance(d Drone, r Request) float64 {
	return math.Sqrt(
		math.Pow(d.X-r.X, 2) +
			math.Pow(d.Y-r.Y, 2),
	)
}

func fulfillRequest(request Request) {
	fmt.Println("Drone indo atender requisição:", request.ID)

	time.Sleep(5 * time.Second)

	fmt.Println("Drone finalizou requisição:", request.ID)
}

func warnDrones(text string, request Request) {
	mu.Lock()
	currentDrones := append([]Drone(nil), drones...)
	currentDrone := drone
	mu.Unlock()

	notice := Notice{
		Text:    text,
		Request: request,
		Drone:   currentDrone,
	}

	for _, d := range currentDrones {
		conn, err := net.DialTimeout("tcp", d.AddressForDrone, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao criar conexão com drone:", err)
			continue
		}

		encoder := json.NewEncoder(conn)

		if err := encoder.Encode(notice); err != nil {
			fmt.Println("Erro ao comunicar com drone:", err)
		}

		_ = conn.Close()
	}
}

func markRequestAsAttending(request Request, attendingDrone Drone) {
	mu.Lock()
	defer mu.Unlock()

	for i := range requests {
		if requests[i].ID == request.ID {
			requests[i].Status = "ATTENDING"
			break
		}
	}

	if drone.ID == attendingDrone.ID {
		drone.IsBusy = true
	}

	for i := range drones {
		if drones[i].ID == attendingDrone.ID {
			drones[i].IsBusy = true
			break
		}
	}
}

func removeRequestDone(request Request, finishedDrone Drone) {
	mu.Lock()
	defer mu.Unlock()

	var filtered []Request

	for _, r := range requests {
		if r.ID == request.ID {
			continue
		}

		filtered = append(filtered, r)
	}

	requests = filtered

	if drone.ID == finishedDrone.ID {
		drone.IsBusy = false
		drone.X = request.X
		drone.Y = request.Y
	}

	for i := range drones {
		if drones[i].ID == finishedDrone.ID {
			drones[i].IsBusy = false
			drones[i].X = request.X
			drones[i].Y = request.Y
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
				bestDistance := calculateDistance(currentDrone, r)

				for _, d := range currentDrones {
					if !d.IsOn || d.IsBusy {
						continue
					}

					tempDistance := calculateDistance(d, r)

					if tempDistance < bestDistance {
						bestDistance = tempDistance
						closer = d
					}

					if tempDistance == bestDistance && d.ID < closer.ID {
						closer = d
					}
				}

				if closer.ID == currentDrone.ID {
					markRequestAsAttending(r, currentDrone)
					warnDrones("ATTENDING", r)

					fulfillRequest(r)

					removeRequestDone(r, currentDrone)
					warnDrones("DONE", r)
				}

				break
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func handleDrones(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	decoder := json.NewDecoder(conn)

	var notice Notice
	if err := decoder.Decode(&notice); err != nil {
		fmt.Println("Erro ao receber aviso do drone:", err)
		return
	}

	if notice.Text == "ATTENDING" {
		markRequestAsAttending(notice.Request, notice.Drone)
		return
	}

	if notice.Text == "DONE" {
		removeRequestDone(notice.Request, notice.Drone)
		return
	}
}

func listenDrones() {
	_, port, _ := net.SplitHostPort(drone.AddressForDrone)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar drone (drone): ", err)
		return
	}
	defer listener.Close()

	fmt.Println("Drone inicializado (drone)")

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

func handleSector(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var request Request

	if err := decoder.Decode(&request); err != nil {
		fmt.Println("Erro ao receber requisição do setor")
		return
	}

	if request.Status != "PENDING" {
		message := Message{Text: "INVALID_COMMAND"}
		_ = encoder.Encode(message)
		return
	}

	mu.Lock()

	exists := false
	for _, r := range requests {
		if r.ID == request.ID {
			exists = true
			break
		}
	}

	if !exists {
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

			if request.Clock == r.Clock &&
				request.ID < r.ID {

				index = i
				break
			}
		}

		requests = append(requests[:index], append([]Request{request}, requests[index:]...)...)
	}

	mu.Unlock()

	_ = encoder.Encode(Message{Text: "QUEUED"})
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

	fmt.Println("Drone inicializado (setor) ")

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
} // Finalizada

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

	if loadDrones(dronesPath, id) != nil {
		return
	}

	go listenSectors()
	go listenDrones()
	go dispatchRequests()

	select {}
}
