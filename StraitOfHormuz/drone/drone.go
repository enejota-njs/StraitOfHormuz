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

type Command struct {
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsCritical bool    `json:"is_critical"`
}

type Drone struct {
	Address string  `json:"address"`
	ID      int     `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	IsBusy  bool    `json:"is_busy"`
	IsOn    bool    `json:"is_on"`
}

type DroneConfig struct {
	Drones []Drone `json:"drones"`
}

var (
	drone Drone
	mu    sync.Mutex
)

// == SETOR

func handleSector(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	for {
		var command Command

		if err := decoder.Decode(&command); err != nil {
			fmt.Println("Erro ao receber comando:", err)
			return
		}

		if command.Type != "REQUEST" {
			if err := encoder.Encode("INVALID_COMMAND"); err != nil {
				fmt.Println("Erro ao responder setor:", err)
				return
			}
			continue
		}

		mu.Lock()

		if drone.IsBusy || !drone.IsOn {
			mu.Unlock()

			if err := encoder.Encode("BUSY"); err != nil {
				fmt.Println("Erro ao responder setor:", err)
				return
			}

			continue
		}

		drone.IsBusy = true
		mu.Unlock()

		if err := encoder.Encode("SERVING"); err != nil {
			fmt.Println("Erro ao responder setor:", err)
			return
		}

		time.Sleep(5 * time.Second)

		mu.Lock()
		drone.IsBusy = false
		mu.Unlock()

		if err := encoder.Encode("FINISHED"); err != nil {
			fmt.Println("Erro ao responder setor:", err)
			return
		}
	}
}

func listenSector(listener net.Listener) {
	fmt.Println("Drone escutando setores...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			continue
		}

		go handleSector(conn)
	}
}

// == REGISTER

func registerDroneID() int {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Digite o ID do drone: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		id, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("ID inválido, digite apenas números")
			continue
		}

		return id
	}
}

func registerDronePort() net.Listener {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Digite o endereço do drone: ")
		address, _ := reader.ReadString('\n')
		address = strings.TrimSpace(address)

		listener, err := net.Listen("tcp", address)
		if err != nil {
			fmt.Println("Endereço inválido ou já em uso")
			continue
		}

		drone.Address = address

		return listener
	}
}

// == SAVE

func saveDrone(path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir drones.json:", err)
		return
	}

	var config DroneConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		_ = file.Close()
		fmt.Println("Erro ao ler drones.json:", err)
		return
	}
	_ = file.Close()

	for _, d := range config.Drones {
		if d.Address == drone.Address {
			fmt.Println("Esse drone já está salvo no arquivo")
			return
		}
	}

	config.Drones = append(config.Drones, drone)

	out, err := os.Create(path)
	if err != nil {
		fmt.Println("Erro ao salvar drones.json:", err)
		return
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(config); err != nil {
		_ = out.Close()
		fmt.Println("Erro ao escrever drones.json:", err)
		return
	}

	_ = out.Close()
}

// == MAIN

func main() {
	dronesPath := "../data/drones.json"

	id := registerDroneID()

	drone = Drone{
		ID:     id,
		X:      0,
		Y:      0,
		IsBusy: false,
		IsOn:   true,
	}

	listener := registerDronePort()
	defer func() {
		_ = listener.Close()
	}()

	saveDrone(dronesPath)

	go listenSector(listener)

	go func() {
		for {
			mu.Lock()
			fmt.Println(drone)
			mu.Unlock()

			time.Sleep(1 * time.Second)
		}
	}()

	select {}
}
