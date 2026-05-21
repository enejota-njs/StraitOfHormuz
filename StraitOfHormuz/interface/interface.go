package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
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
		if stat, _ := file.Stat(); stat.Size() > 0 {
			if err := json.NewDecoder(file).Decode(&list); err != nil {
				file.Close()
				return fmt.Errorf("arquivo ocupado")
			}
		}
		file.Close()
	}

	var filtered []Drone
	exists := false

	for _, d := range list {
		if d.ID == drone.ID {
			exists = true
			if drone.IsOn {
				filtered = append(filtered, drone)
			}
		} else {
			filtered = append(filtered, d)
		}
	}

	if !exists && drone.IsOn {
		filtered = append(filtered, drone)
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(filtered)
}

func saveSector(path string, sector Sector) error {
	var list []Sector

	file, err := os.Open(path)
	if err == nil {
		if stat, _ := file.Stat(); stat.Size() > 0 {
			if err := json.NewDecoder(file).Decode(&list); err != nil {
				file.Close()
				return fmt.Errorf("arquivo ocupado")
			}
		}
		file.Close()
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

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(list)
}

func saveSensor(path string, sensor Sensor) error {
	var list []Sensor

	file, err := os.Open(path)
	if err == nil {
		if stat, _ := file.Stat(); stat.Size() > 0 {
			if err := json.NewDecoder(file).Decode(&list); err != nil {
				file.Close()
				return fmt.Errorf("arquivo ocupado")
			}
		}
		file.Close()
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

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(list)
}

func saveRequest(path string, request Request) error {
	var list []Request

	file, err := os.Open(path)
	if err == nil {
		if stat, _ := file.Stat(); stat.Size() > 0 {
			if err := json.NewDecoder(file).Decode(&list); err != nil {
				file.Close()
				return fmt.Errorf("arquivo ocupado")
			}
		}
		file.Close()
	}

	var filtered []Request
	exists := false

	for _, r := range list {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			if request.Status == "DONE" {
				exists = true
				continue
			}
			filtered = append(filtered, request)
			exists = true
		} else {
			filtered = append(filtered, r)
		}
	}

	if !exists && request.Status != "DONE" {
		filtered = append(filtered, request)
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(filtered)
}

func listenDrones(port string, path string) {
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

		go func(conn net.Conn) {
			defer conn.Close()
			var drone Drone

			if err := json.NewDecoder(conn).Decode(&drone); err != nil {
				return
			}

			mu.Lock()
			_ = saveDrone(path, drone)
			mu.Unlock()
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

	fmt.Println("Servidor inicializado (setor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			var sector Sector
			if err := json.NewDecoder(conn).Decode(&sector); err != nil {
				return
			}
			mu.Lock()
			_ = saveSector(path, sector)
			mu.Unlock()
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

	fmt.Println("Servidor inicializado (sensor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			var sensor Sensor
			if err := json.NewDecoder(conn).Decode(&sensor); err != nil {
				return
			}
			mu.Lock()
			_ = saveSensor(path, sensor)
			mu.Unlock()
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

	fmt.Println("Servidor inicializado (requisição)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			var request Request
			if err := json.NewDecoder(conn).Decode(&request); err != nil {
				return
			}
			mu.Lock()
			_ = saveRequest(path, request)
			mu.Unlock()
		}(conn)
	}
}

// == LOAD DATA

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

// == MAIN

func main() {
	clearFile("data/interface/drones.json")
	clearFile("data/interface/sectors.json")
	clearFile("data/interface/sensors.json")
	clearFile("data/interface/requests.json")

	interfacePath := "data/initialization/interface.json"

	sectorsPort, dronesPort, sensorsPort, requestsPort, err := loadInterfacePorts(interfacePath)
	if err != nil {
		fmt.Println("Erro ao ler interface.json:", err)
		return
	}

	_, sectorsPort, _ = net.SplitHostPort(sectorsPort)
	_, dronesPort, _ = net.SplitHostPort(dronesPort)
	_, sensorsPort, _ = net.SplitHostPort(sensorsPort)
	_, requestsPort, _ = net.SplitHostPort(requestsPort)

	go listenDrones(dronesPort, "data/interface/drones.json")
	go listenSectors(sectorsPort, "data/interface/sectors.json")
	go listenSensors(sensorsPort, "data/interface/sensors.json")
	go listenRequests(requestsPort, "data/interface/requests.json")

	select {}
}
