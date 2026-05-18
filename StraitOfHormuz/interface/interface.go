package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
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
	ID               int     `json:"id"`
	AddressForSector string  `json:"address_for_sector"`
	AddressForDrone  string  `json:"address_for_drone"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	IsBusy           bool    `json:"is_busy"`
	IsOn             bool    `json:"is_on"`
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

type Sensor struct {
	ID         int     `json:"id"`
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsActive   bool    `json:"is_active"`
	IsCritical bool    `json:"is_critical"`
}

var mu sync.Mutex

func saveDrone(path string, drone Drone) error {
	var list []Drone

	file, err := os.Open(path)
	if err == nil {
		_ = json.NewDecoder(file).Decode(&list)
		_ = file.Close()
	}

	exists := false

	for i := range list {
		if list[i].ID == drone.ID {
			list[i] = drone
			exists = true
			break
		}
	}

	if !exists {
		list = append(list, drone)
	}

	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer output.Close()

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	return encoder.Encode(list)
}

func saveSector(path string, sector Sector) error {
	var list []Sector

	file, err := os.Open(path)
	if err == nil {
		_ = json.NewDecoder(file).Decode(&list)
		_ = file.Close()
	}

	exists := false

	for i := range list {
		if list[i].ID == sector.ID {
			list[i] = sector
			exists = true
			break
		}
	}

	if !exists {
		list = append(list, sector)
	}

	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer output.Close()

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	return encoder.Encode(list)
}

func saveSensor(path string, sensor Sensor) error {
	var list []Sensor

	file, err := os.Open(path)
	if err == nil {
		_ = json.NewDecoder(file).Decode(&list)
		_ = file.Close()
	}

	exists := false

	for i := range list {
		if list[i].Type == sensor.Type && list[i].X == sensor.X && list[i].Y == sensor.Y {
			list[i] = sensor
			exists = true
			break
		}
	}

	if !exists {
		list = append(list, sensor)
	}

	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer output.Close()

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	return encoder.Encode(list)
}

func listenDrones(port string, path string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos drones:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor recebendo drones na porta", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var drone Drone

			if err := json.NewDecoder(conn).Decode(&drone); err != nil {
				fmt.Println("Erro ao receber drone:", err)
				return
			}

			mu.Lock()
			err := saveDrone(path, drone)
			mu.Unlock()

			if err != nil {
				fmt.Println("Erro ao salvar drone:", err)
			}
		}(conn)
	}
}

func listenSectors(port string, path string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos setores:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor recebendo setores na porta", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var sector Sector

			if err := json.NewDecoder(conn).Decode(&sector); err != nil {
				fmt.Println("Erro ao receber setor:", err)
				return
			}

			mu.Lock()
			err := saveSector(path, sector)
			mu.Unlock()

			if err != nil {
				fmt.Println("Erro ao salvar setor:", err)
			}
		}(conn)
	}
}

func listenSensors(port string, path string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos sensores:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor recebendo sensores na porta", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var sensor Sensor

			if err := json.NewDecoder(conn).Decode(&sensor); err != nil {
				fmt.Println("Erro ao receber sensor:", err)
				return
			}

			mu.Lock()
			err := saveSensor(path, sensor)
			mu.Unlock()

			if err != nil {
				fmt.Println("Erro ao salvar sensor:", err)
			}
		}(conn)
	}
}

func listenRequests(port string, path string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta das requisições:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor recebendo requisições na porta", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var request Request

			if err := json.NewDecoder(conn).Decode(&request); err != nil {
				fmt.Println("Erro ao receber requisição:", err)
				return
			}

			mu.Lock()
			err := saveRequest(path, request)
			mu.Unlock()

			if err != nil {
				fmt.Println("Erro ao salvar requisição:", err)
			}
		}(conn)
	}
}

func saveRequest(path string, request Request) error {
	var list []Request

	file, err := os.Open(path)
	if err == nil {
		_ = json.NewDecoder(file).Decode(&list)
		_ = file.Close()
	}

	var filtered []Request
	exists := false

	// Filtra a lista
	for i := range list {
		if list[i].SectorID == request.SectorID && list[i].ID == request.ID {
			// Se o status for DONE, simplesmente não adicionamos à filtered (removemos)
			if request.Status == "DONE" {
				exists = true
				continue
			}

			// Se não for DONE, atualizamos
			list[i] = request
			filtered = append(filtered, list[i])
			exists = true
		} else {
			filtered = append(filtered, list[i])
		}
	}

	// Se for nova e não for DONE, adiciona
	if !exists && request.Status != "DONE" {
		filtered = append(filtered, request)
	}

	output, err := os.Create(path)
	if err != nil {
		return err
	}
	defer output.Close()

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	return encoder.Encode(filtered)
}

func loadInterfacePorts(path string) (string, string, string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", "", "", err
	}
	defer file.Close()

	var config []struct {
		Sectors  string `json:"sectors"`
		Drones   string `json:"drones"`
		Sensors  string `json:"sensors"`
		Requests string `json:"requests"`
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return "", "", "", "", err
	}

	return config[0].Sectors, config[0].Drones, config[0].Sensors, config[0].Requests, nil
}

func clearFile(path string) {
	file, err := os.Create(path)
	if err != nil {
		fmt.Println("Erro ao limpar arquivo:", path, err)
		return
	}
	defer file.Close()

	_, _ = file.WriteString("[]")
}

func main() {
	clearFile("../data/interface/drones.json")
	clearFile("../data/interface/sectors.json")
	clearFile("../data/interface/sensors.json")
	clearFile("../data/interface/requests.json")

	interfacePath := "../data/initialization/interface.json"

	sectorsPort, dronesPort, sensorsPort, requestsPort, err := loadInterfacePorts(interfacePath)
	if err != nil {
		fmt.Println("Erro ao ler interface.json:", err)
		return
	}

	_, sectorsPort, _ = net.SplitHostPort(sectorsPort)
	_, dronesPort, _ = net.SplitHostPort(dronesPort)
	_, sensorsPort, _ = net.SplitHostPort(sensorsPort)
	_, requestsPort, _ = net.SplitHostPort(requestsPort)

	go listenDrones(dronesPort, "../data/interface/drones.json")
	go listenSectors(sectorsPort, "../data/interface/sectors.json")
	go listenSensors(sensorsPort, "../data/interface/sensors.json")
	go listenRequests(requestsPort, "../data/interface/requests.json")

	select {}
}
