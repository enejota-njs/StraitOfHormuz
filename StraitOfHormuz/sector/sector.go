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
	SectorID   int     `json:"sector_id"`
	ID         int     `json:"origin_id"`
	Status     string  `json:"status"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsCritical bool    `json:"is_critical"`
	Clock      int     `json:"clock"`
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
	Text    string  `json:"text"`
	Request Request `json:"request"`
	Clock   int     `json:"clock"`
	Drone   Drone   `json:"drone"`
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
			fmt.Println("[QUEUE] Requisição já existe, não adicionou")
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

	fmt.Println("\n[REQUEST] Nova requisição criada")
	fmt.Println("  SectorID:", request.SectorID)
	fmt.Println("  RequestID:", request.ID)
	fmt.Println("  X:", request.X, "Y:", request.Y)
	fmt.Println("  Critical:", request.IsCritical)
	fmt.Println("  Clock:", request.Clock)

	addRequestToQueue(request)

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
			fmt.Println("Setor indisponível: ID ", s.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		fmt.Println("[REQUEST] Enviando requisição para setor ID:", s.ID, "Endereço:", s.AddressForSector)

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

		fmt.Println("[REQUEST] Resposta do setor ID:", s.ID, "Texto:", response.Text, "Clock:", response.Clock)

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "QUEUED" {
			fmt.Println("[REQUEST] Requisição confirmada e listada")
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

		fmt.Println("[REQUEST] Enviando requisição para drone ID:", d.ID, "Endereço:", d.AddressForSector)

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

		fmt.Println("[REQUEST] Resposta do drone ID:", d.ID, "Texto:", response.Text, "Clock:", response.Clock)

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "QUEUED" {
			fmt.Println("Listada")
			_ = conn.Close()
		}
	}
}

// == DRONE

func markRequestAsAttending(request Request, attendingDrone Drone) {
	fmt.Println("\n[ATTENDING] Drone aceitou requisição")
	fmt.Println("  DroneID:", attendingDrone.ID)
	fmt.Println("  SectorID:", request.SectorID)
	fmt.Println("  RequestID:", request.ID)

	for i := range requests {
		if requests[i].SectorID == request.SectorID && requests[i].ID == request.ID {
			fmt.Println("[ATTENDING] Status antes:", requests[i].Status)
			requests[i].Status = "ATTENDING"
			fmt.Println("[ATTENDING] Status depois:", requests[i].Status)
			break
		}
	}
}

func removeRequestDone(request Request) {
	fmt.Println("\n[DONE] Removendo requisição finalizada")
	fmt.Println("  SectorID:", request.SectorID)
	fmt.Println("  RequestID:", request.ID)

	var filtered []Request

	for _, r := range requests {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			fmt.Println("[DONE] Requisição encontrada e removida")
			continue
		}

		filtered = append(filtered, r)
	}

	requests = filtered

	fmt.Println("[DONE] Fila após remoção:")
	for i, r := range requests {
		fmt.Println(" ", i, "-> Sector:", r.SectorID, "ID:", r.ID, "Status:", r.Status)
	}
}

func handleDrone(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if decoder.Decode(&message) != nil {
		return
	}

	fmt.Println("\n[DRONE MSG] Mensagem recebida de drone")
	fmt.Println("  Text:", message.Text)
	fmt.Println("  Clock recebido:", message.Clock)
	fmt.Println("  DroneID:", message.Drone.ID)
	fmt.Println("  Request SectorID:", message.Request.SectorID)
	fmt.Println("  Request ID:", message.Request.ID)

	switch message.Text {
	case "ATTENDING":
		fmt.Println("[DRONE MSG] Processando ATTENDING")

		mu.Lock()
		currentClock := updateClock(message.Clock)
		markRequestAsAttending(message.Request, message.Drone)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})

	case "DONE":
		fmt.Println("[DRONE MSG] Processando DONE")

		mu.Lock()
		currentClock := updateClock(message.Clock)
		removeRequestDone(message.Request)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "REMOVED",
			Clock: currentClock,
		})
	}
}

func listenDrone() {
	_, port, _ := net.SplitHostPort(sector.AddressForDrone)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (drone): ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (drone)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar com drone: ", err)
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
			fmt.Println("Erro ao receber sensor: ", err)
			_ = conn.Close()
			return
		}

		fmt.Println("\n[SENSOR] Sensor recebido")
		fmt.Println("  ID:", sensor.ID)
		fmt.Println("  Type:", sensor.Type)
		fmt.Println("  X:", sensor.X, "Y:", sensor.Y)
		fmt.Println("  Active:", sensor.IsActive)
		fmt.Println("  Critical:", sensor.IsCritical)

		if sensor.IsActive {
			fmt.Println("[SENSOR] Sensor está ativo, criando requisição")

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

		fmt.Println("Sensor conectado")

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

	fmt.Println("\n[SECTOR MSG] Mensagem recebida de outro setor")
	fmt.Println("  Text:", message.Text)
	fmt.Println("  Clock recebido:", message.Clock)
	fmt.Println("  Request SectorID:", message.Request.SectorID)
	fmt.Println("  Request ID:", message.Request.ID)

	switch message.Text {
	case "REQUEST":
		fmt.Println("[SECTOR MSG] Processando REQUEST e adicionando na fila")

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

// == LOAD DATA

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

// == SAVE DATA

func sendSectorToInterface(path string) {
	mu.Lock()
	currentSector := sector
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

	fmt.Println("\n[INTERFACE] Enviando setor para interface")
	fmt.Println("  SectorID:", currentSector.ID)
	fmt.Println("  Endereço interface setores:", config[0].Sectors)

	for {
		conn, err := net.DialTimeout("tcp", config[0].Sectors, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao conectar com a interface: ", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if err = json.NewEncoder(conn).Encode(currentSector); err != nil {
			fmt.Println("Erro ao enviar setor para interface:", err)
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}

	fmt.Println("[INTERFACE] Setor enviado com sucesso")
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

	fmt.Println("\n[MAIN] Setor iniciado")
	fmt.Println("  ID:", sector.ID)
	fmt.Println("  AddressForSensor:", sector.AddressForSensor)
	fmt.Println("  AddressForSector:", sector.AddressForSector)
	fmt.Println("  AddressForDrone:", sector.AddressForDrone)

	fmt.Println("[MAIN] Outros setores carregados:", len(sectors))
	for _, s := range sectors {
		fmt.Println("  Setor ID:", s.ID, "|", s.AddressForSector)
	}

	fmt.Println("[MAIN] Drones carregados:", len(drones))
	for _, d := range drones {
		fmt.Println("  Drone ID:", d.ID, "|", d.AddressForSector)
	}

	go sendSectorToInterface(intefacePath)

	go listenSensor()
	go listenSectors()
	go listenDrone()

	select {}
}
