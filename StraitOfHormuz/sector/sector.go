package main

import (
	"bufio"
	"encoding/json"
	"fmt"
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

type Drone struct {
	Address string  `json:"address"`
	ID      int     `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	IsBusy  bool    `json:"is_busy"`
	IsOn    bool    `json:"is_on"`
}

type SectorConfig struct {
	Sectors []Sector `json:"sectors.json"`
}

type DroneConfig struct {
	Drones []Drone `json:"drones"`
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

// == SECTOR

func treatSector(conn net.Conn) {
	if conn != nil {
		_ = conn.Close()
	}
}

func addSectorLocal(newSector Sector) {
	if newSector.AddressForSector == sector.AddressForSector {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	for _, s := range sectors {
		if s.AddressForSector == newSector.AddressForSector {
			return
		}
	}

	sectors = append(sectors, newSector)
	fmt.Println("Novo setor adicionado na lista local:", newSector.AddressForSector)
}

func monitorSector(address string) {
	for {
		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err != nil {
			fmt.Println("Setor indisponível:", address)
			time.Sleep(2 * time.Second)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		for {
			if err := encoder.Encode("PING"); err != nil {
				fmt.Println("Erro ao enviar PING para:", address)
				treatSector(conn)
				break
			}

			var message string
			if err := decoder.Decode(&message); err != nil {
				fmt.Println("Erro ao receber PONG de:", address)
				treatSector(conn)
				break
			}

			if message != "PONG" {
				fmt.Println("Resposta inválida de:", address)
				treatSector(conn)
				break
			}

			time.Sleep(5 * time.Second)
		}
	}
}

func checkSector() {
	alreadyMonitoring := make(map[string]bool)

	for {
		mu.Lock()
		current := make([]Sector, len(sectors))
		copy(current, sectors)
		mu.Unlock()

		for _, s := range current {
			if s.AddressForSector == sector.AddressForSector {
				continue
			}

			if !alreadyMonitoring[s.AddressForSector] {
				alreadyMonitoring[s.AddressForSector] = true
				go monitorSector(s.AddressForSector)
			}
		}

		time.Sleep(2 * time.Second)
	}
}

func handleSector(conn net.Conn) {
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	for {
		var message string

		if err := decoder.Decode(&message); err != nil {
			treatSector(conn)
			return
		}

		if message == "PING" {
			if err := encoder.Encode("PONG"); err != nil {
				treatSector(conn)
				return
			}
			continue
		}

		var newSector Sector
		if err := json.Unmarshal([]byte(message), &newSector); err != nil {
			fmt.Println("Mensagem incorreta do setor")
			treatSector(conn)
			return
		}

		addSectorLocal(newSector)

		if err := encoder.Encode("OK"); err != nil {
			treatSector(conn)
			return
		}
	}
}

func notifySectorsMyAddress() {
	mu.Lock()
	current := make([]Sector, len(sectors))
	copy(current, sectors)
	mu.Unlock()

	for _, s := range current {
		if s.AddressForSector == sector.AddressForSector {
			continue
		}

		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao avisar setor:", s.AddressForSector)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		data, err := json.Marshal(sector)
		if err != nil {
			fmt.Println("Erro ao converter setor para JSON:", err)
			_ = conn.Close()
			continue
		}

		if err := encoder.Encode(string(data)); err != nil {
			fmt.Println("Erro ao enviar endereço para setor:", s.AddressForSector)
			_ = conn.Close()
			continue
		}

		var response string
		if err := decoder.Decode(&response); err != nil {
			fmt.Println("Erro ao receber confirmação do setor:", s.AddressForSector)
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
	}
}

func removeSelfFromSectors() {
	var result []Sector

	for _, s := range sectors {
		if s.AddressForSector != sector.AddressForSector {
			result = append(result, s)
		}
	}

	sectors = result
}

func listenSector(listener net.Listener) {
	fmt.Println("Servidor (setor) inicializado")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar com setor:", err)
			continue
		}

		go handleSector(conn)
	}
}

// == SENSOR

func handleSensor(conn net.Conn) {
	decoder := json.NewDecoder(conn)
	var sensor Sensor

	for {
		if err := decoder.Decode(&sensor); err != nil {
			fmt.Println("Erro ao receber sensor:", err)
			_ = conn.Close()
			return
		}

		if sensor.IsActive {
			go requestDrone(sensor)
		}
	}
}

func listenSensor(listener net.Listener) {
	fmt.Println("Servidor (sensor) inicializado")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar com sensor:", err)
			continue
		}

		go handleSensor(conn)
	}
}

// == LOAD DATA

func loadSectors(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	var config SectorConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	mu.Lock()
	sectors = config.Sectors
	mu.Unlock()

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

	var config DroneConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	mu.Lock()
	drones = config.Drones
	mu.Unlock()

	return nil
}

// == SAVE DATA

func saveSector(path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir sectors.json", err)
		return
	}

	var config SectorConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		_ = file.Close()
		fmt.Println("Erro ao ler sectors.json:", err)
		return
	}
	_ = file.Close()

	for _, s := range config.Sectors {
		if s.AddressForSector == sector.AddressForSector {
			fmt.Println("Esse setor já está salvo no arquivo")
			return
		}
	}

	config.Sectors = append(config.Sectors, sector)

	out, err := os.Create(path)
	if err != nil {
		fmt.Println("Erro ao salvar sectors.json:", err)
		return
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(config); err != nil {
		_ = out.Close()
		fmt.Println("Erro ao escrever sectors.json:", err)
		return
	}

	_ = out.Close()
}

// == REGISTER

func registerSectorPort() net.Listener {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Digite o endereço do setor: ")
		address, _ := reader.ReadString('\n')
		address = strings.TrimSpace(address)

		listener, err := net.Listen("tcp", address)
		if err != nil {
			fmt.Println("Essa porta/endereço já está em uso ou é inválido")
			continue
		}

		sector.AddressForSector = address

		return listener
	}
}

func registerSensorPort() net.Listener {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Digite o endereço para escutar sensores: ")
		address, _ := reader.ReadString('\n')
		address = strings.TrimSpace(address)

		listener, err := net.Listen("tcp", address)
		if err != nil {
			fmt.Println("Essa porta/endereço já está em uso ou é inválido")
			continue
		}

		sector.AddressForSensor = address

		return listener
	}
}

func registerSector() {
	reader := bufio.NewReader(os.Stdin)

	var left, right, top, bottom float64

	for {
		fmt.Print("Digite o X da esquerda: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			continue
		}

		left = val
		break
	}

	for {
		fmt.Print("Digite o X da direita: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			continue
		}

		if val <= left {
			fmt.Println("O X da direita deve ser maior que o X da esquerda")
			continue
		}

		right = val
		break
	}

	for {
		fmt.Print("Digite o Y de cima: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			continue
		}

		top = val
		break
	}

	for {
		fmt.Print("Digite o Y de baixo: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			continue
		}

		if val >= top {
			fmt.Println("O Y de baixo deve ser menor que o Y de cima")
			continue
		}

		bottom = val
		break
	}

	sector.Left = left
	sector.Right = right
	sector.Top = top
	sector.Bottom = bottom
}

// == MAIN

func main() {
	sectorsPath := "../data/sectors.json"
	dronesPath := "../data/drones.json"

	registerSector()

	listenerSector := registerSectorPort()
	defer func() {
		_ = listenerSector.Close()
	}()

	listenerSensor := registerSensorPort()
	defer func() {
		_ = listenerSensor.Close()
	}()

	saveSector(sectorsPath)

	if err := loadSectors(sectorsPath); err != nil {
		panic(err)
	}

	removeSelfFromSectors()
	notifySectorsMyAddress()

	if err := loadDrones(dronesPath); err != nil {
		panic(err)
	}

	go func() {
		for {
			if err := loadDrones(dronesPath); err != nil {
				fmt.Println("Erro ao carregar drones:", err)
			}

			time.Sleep(5 * time.Second)
		}
	}()

	go listenSector(listenerSector)
	go listenSensor(listenerSensor)
	go checkSector()

	select {}
}
