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
	fmt.Println("[CLOCK] Incrementado para:", clock)

	return clock
}

func updateClock(receivedClock int) int {
	fmt.Println("[CLOCK] Recebido:", receivedClock, "| Local antes:", clock)

	if receivedClock > clock {
		clock = receivedClock
	}

	incrementClock()

	fmt.Println("[CLOCK] Local depois:", clock)
	return clock
}

// == REQUEST

func addRequestToQueue(request Request) {
	fmt.Println("[QUEUE] Tentando adicionar requisição:")
	fmt.Println("  SectorID:", request.SectorID)
	fmt.Println("  RequestID:", request.ID)
	fmt.Println("  Status:", request.Status)
	fmt.Println("  Critical:", request.IsCritical)
	fmt.Println("  Clock:", request.Clock)

	for _, r := range requests {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			fmt.Println("[QUEUE] Requisição já existe")
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

	fmt.Println("[QUEUE] Requisição adicionada na posição:", index)
	fmt.Println("[QUEUE] Fila atual:")
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
			fmt.Println("[SYNC] Setor indisponível:", s.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err := encoder.Encode(message); err != nil {
			fmt.Println("[SYNC] Erro ao pedir fila ao setor:", s.ID)
			_ = conn.Close()
			continue
		}

		var response Message
		if err := decoder.Decode(&response); err != nil {
			fmt.Println("[SYNC] Erro ao receber fila do setor:", s.ID)
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)

		for _, r := range response.Requests {
			addRequestToQueue(r)
		}

		mu.Unlock()

		fmt.Println("[SYNC] Fila sincronizada com setor:", s.ID)

		_ = conn.Close()
	}
}

// == DRONE

func calculateDistance(d Drone, r Request) float64 {
	distance := math.Sqrt(
		math.Pow(d.X-r.X, 2) +
			math.Pow(d.Y-r.Y, 2),
	)

	fmt.Println(
		"[DISTANCE] Drone", d.ID,
		"-> Request", r.ID,
		"Distância:", distance,
	)

	return distance
}

func fulfillRequest(request Request) {
	fmt.Println("\n[MISSION] Drone iniciou atendimento")
	fmt.Println("  DroneID:", drone.ID)
	fmt.Println("  RequestID:", request.ID)
	fmt.Println("  Destino:", request.X, request.Y)

	steps := 50
	delay := 100 * time.Millisecond

	for i := 1; i <= steps; i++ {
		mu.Lock()

		dx := request.X - drone.X
		dy := request.Y - drone.Y

		drone.X += dx / float64(steps-i+1)
		drone.Y += dy / float64(steps-i+1)

		mu.Unlock()

		fmt.Println(
			"[MISSION] Drone movendo",
			"| X:", drone.X,
			"| Y:", drone.Y,
		)

		time.Sleep(delay)
	}

	mu.Lock()
	drone.X = request.X
	drone.Y = request.Y
	mu.Unlock()

	fmt.Println("\n[MISSION] Drone chegou")
	fmt.Println("  X:", drone.X)
	fmt.Println("  Y:", drone.Y)

	fmt.Println("[MISSION] Simulando atendimento...")
	time.Sleep(10 * time.Second)

	fmt.Println("[MISSION] Atendimento finalizado")
}

func warnDrones(text string, request Request) {
	mu.Lock()
	clockValue := incrementClock()
	currentDrones := append([]Drone(nil), drones...)
	currentDrone := drone
	mu.Unlock()

	fmt.Println("\n[DRONE BROADCAST] Enviando mensagem para drones")
	fmt.Println("  Tipo:", text)
	fmt.Println("  RequestID:", request.ID)
	fmt.Println("  Clock:", clockValue)

	message := Message{
		Text:    text,
		Request: request,
		Drone:   currentDrone,
		Clock:   clockValue,
	}

	for _, d := range currentDrones {
		fmt.Println("[DRONE BROADCAST] Conectando no drone:", d.ID)

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

		fmt.Println("[DRONE BROADCAST] Mensagem enviada para drone:", d.ID)

		var response Message

		if err = decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber resposta do drone:", d.ID)
			_ = conn.Close()
			continue
		}

		fmt.Println(
			"[DRONE BROADCAST] Resposta recebida",
			"| Drone:", d.ID,
			"| Texto:", response.Text,
			"| Clock:", response.Clock,
		)

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
	fmt.Println("\n[ATTENDING] Atualizando requisição")
	fmt.Println("  RequestID:", request.ID)
	fmt.Println("  Drone responsável:", attendingDrone.ID)

	mu.Lock()
	defer mu.Unlock()

	for i := range requests {
		if requests[i].ID == request.ID && requests[i].SectorID == request.SectorID {
			requests[i].Status = "ATTENDING"

			fmt.Println(
				"[ATTENDING] Status alterado para ATTENDING",
				"| Request:", requests[i].ID,
			)
			break
		}
	}

	if drone.ID == attendingDrone.ID {
		drone.IsBusy = true
		fmt.Println("[ATTENDING] Meu drone agora está ocupado")
		return
	}

	for i := range drones {
		if drones[i].ID == attendingDrone.ID {
			drones[i].IsBusy = true
			drones[i].X = attendingDrone.X
			drones[i].Y = attendingDrone.Y

			fmt.Println(
				"[ATTENDING] Drone remoto atualizado",
				"| Drone:", drones[i].ID,
				"| Busy:", drones[i].IsBusy,
			)
			break
		}
	}

	request.Status = "ATTENDING"
	go sendRequestToInterface("../data/initialization/interface.json", request)
}

func removeRequestDone(request Request, finishedDrone Drone) {
	fmt.Println("\n[DONE] Finalizando requisição")
	fmt.Println("  RequestID:", request.ID)
	fmt.Println("  Drone:", finishedDrone.ID)

	mu.Lock()
	defer mu.Unlock()

	var filtered []Request

	for _, r := range requests {
		if r.ID == request.ID && r.SectorID == request.SectorID {
			fmt.Println("[DONE] Requisição removida da fila")
			continue
		}

		filtered = append(filtered, r)
	}

	requests = filtered

	if drone.ID == finishedDrone.ID {
		drone.IsBusy = false
		drone.X = finishedDrone.X
		drone.Y = finishedDrone.Y

		fmt.Println("[DONE] Meu drone agora está livre")
	}

	for i := range drones {
		if drones[i].ID == finishedDrone.ID {
			drones[i].IsBusy = false
			drones[i].X = finishedDrone.X
			drones[i].Y = finishedDrone.Y

			fmt.Println(
				"[DONE] Drone remoto atualizado",
				"| Drone:", drones[i].ID,
				"| Busy:", drones[i].IsBusy,
			)
			break
		}
	}

	fmt.Println("[DONE] Fila restante:")
	for _, r := range requests {
		fmt.Println(
			"  Request:",
			r.ID,
			"| Sector:",
			r.SectorID,
			"| Status:",
			r.Status,
		)
	}

	request.Status = "DONE"
	go sendRequestToInterface("../data/initialization/interface.json", request)
}

func dispatchRequests() {
	for {
		mu.Lock()

		currentDrone := drone
		currentRequests := append([]Request(nil), requests...)
		currentDrones := append([]Drone(nil), drones...)

		mu.Unlock()

		fmt.Println("\n[DISPATCH] Verificando requisições")
		fmt.Println("  Drone:", currentDrone.ID)
		fmt.Println("  Busy:", currentDrone.IsBusy)
		fmt.Println("  Requests:", len(currentRequests))

		if currentDrone.IsOn && !currentDrone.IsBusy {
			for _, r := range currentRequests {
				if r.Status != "PENDING" {
					continue
				}

				fmt.Println(
					"[DISPATCH] Analisando requisição",
					"| Request:", r.ID,
					"| Sector:", r.SectorID,
				)

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
					"\n[DISPATCH] Drone escolhido",
					"| Drone:", closer.ID,
					"| Request:", r.ID,
					"| Sector:", r.SectorID,
				)

				if closer.ID == currentDrone.ID {
					markRequestAsAttending(r, currentDrone)

					warnDrones("ATTENDING", r)
					warnSectors("ATTENDING", r)

					fmt.Println("[DISPATCH] Iniciando missão")
					fulfillRequest(r)
					fmt.Println("[DISPATCH] Missão concluída")

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

	fmt.Println("\n[DRONE MSG] Mensagem recebida")
	fmt.Println("  Text:", message.Text)
	fmt.Println("  Drone origem:", message.Drone.ID)
	fmt.Println("  RequestID:", message.Request.ID)
	fmt.Println("  Clock:", message.Clock)

	if message.Text == "ATTENDING" {
		fmt.Println("[DRONE MSG] Processando ATTENDING")

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
		fmt.Println("[DRONE MSG] Processando DONE")

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

		fmt.Println("[SERVER] Nova conexão de drone aceita")

		go handleDrones(conn)
	}
}

// == SECTOR

func warnSectors(text string, request Request) {
	fmt.Println("\n[SECTOR BROADCAST] Enviando atualização")
	fmt.Println("  Tipo:", text)
	fmt.Println("  Request:", request.ID)

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
		fmt.Println("[SECTOR BROADCAST] Conectando setor:", s.ID)

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

		fmt.Println("[SECTOR BROADCAST] Mensagem enviada")

		var response Message

		if err = decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber resposta do setor:", s.ID)
			_ = conn.Close()
			continue
		}

		fmt.Println(
			"[SECTOR BROADCAST] Resposta",
			"| Setor:", s.ID,
			"| Texto:", response.Text,
		)

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

	fmt.Println("\n[SECTOR MSG] Mensagem recebida")
	fmt.Println("  Text:", message.Text)
	fmt.Println("  Request:", message.Request.ID)
	fmt.Println("  Sector origem:", message.Request.SectorID)
	fmt.Println("  Clock:", message.Clock)

	switch message.Text {
	case "REQUEST":
		fmt.Println("[SECTOR MSG] Adicionando requisição na fila")

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

		fmt.Println("[SERVER] Nova conexão de setor aceita")

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

func sendRequestToInterface(path string, request Request) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("[INTERFACE REQUEST] Erro ao abrir interface.json:", err)
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
		fmt.Println("[INTERFACE REQUEST] Erro ao ler interface.json:", err)
		return
	}

	conn, err := net.DialTimeout("tcp", config[0].Requests, 2*time.Second)
	if err != nil {
		fmt.Println("[INTERFACE REQUEST] Interface indisponível:", err)
		return
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		fmt.Println("[INTERFACE REQUEST] Erro ao enviar request:", err)
		return
	}

	fmt.Println("[INTERFACE REQUEST] Request enviada/atualizada na interface")
}

func sendDroneToInterface(path string) {
	mu.Lock()
	currentDrone := drone
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
		conn, err := net.DialTimeout("tcp", config[0].Drones, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao conectar com a interface: ", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if err := json.NewEncoder(conn).Encode(currentDrone); err != nil {
			fmt.Println("Erro ao enviar drone para interface:", err)
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}
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

	fmt.Println("\n[MAIN] Drone iniciado")
	fmt.Println("  ID:", drone.ID)
	fmt.Println("  X:", drone.X)
	fmt.Println("  Y:", drone.Y)
	fmt.Println("  Busy:", drone.IsBusy)
	fmt.Println("  On:", drone.IsOn)

	fmt.Println("[MAIN] Outros drones:", len(drones))
	fmt.Println("[MAIN] Setores:", len(sectors))

	go listenDrones()
	go listenSectors()

	syncRequestsFromSectors()

	go dispatchRequests()

	go sendDroneLoop(intefacePath)

	select {}
}
