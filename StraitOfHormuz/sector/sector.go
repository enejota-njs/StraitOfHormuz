package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

type Sensor struct {
	Type       string `json:"type"`
	Longitude  int    `json:"longitude"`
	IsActive   bool   `json:"is_active"`
	IsCritical bool   `json:"is_critical"`
}

type Sector struct {
	LonMin float64 `json:"lon_min"`
	LonMax float64 `json:"lon_max"`
}

func sendMessage(encoder *json.Encoder, message string) error {
	err := encoder.Encode(message)

	if err != nil {
		fmt.Println("Erro ao enviar mensagem: ", err)
		return err
	}

	return nil
}

func receiveMessage(decoder *json.Decoder, message *string) error {
	err := decoder.Decode(message)

	if err != nil {
		fmt.Println("Erro ao receber mensagem: ", err)
		return err
	}

	return nil
}

func receiveSensor(decoder *json.Decoder, sensor *Sensor) error {
	err := decoder.Decode(sensor)

	if err != nil {
		fmt.Println("Erro ao receber sensor: ", err)
		return err
	}

	return nil
}

func requestDrone(sensor Sensor) {

}

func handleSensor(conn net.Conn) {
	decoder := json.NewDecoder(conn)

	for {
		var sensor Sensor
		if receiveSensor(decoder, &sensor) != nil {
			conn.Close()
		}

		if sensor.IsActive {
			requestDrone(sensor)
		}
	}
}

func listenSensor() {
	listener, err := net.Listen("tcp", "localhost:8000")
	if err != nil {
		fmt.Println("Erro ao iniciar porta 8000: ", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar com sensor: ", err)
			return
		}

		go handleSensor(conn)
	}
}

// == SECTOR

func treatSector(conn net.Conn) {

}

func monitorSector(address string) {
	for {
		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err != nil {
			fmt.Printf("Setor (" + address + ") indisponível\n")
			treatSector(conn)
			time.Sleep(2 * time.Second)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		for {
			if sendMessage(encoder, "PING") != nil {
				treatSector(conn)
				break
			}

			var message string
			if receiveMessage(decoder, &message) != nil {
				treatSector(conn)
				break
			}

			if message != "PONG" {
				treatSector(conn)
				break
			}

			time.Sleep(5 * time.Second)
		}
	}
}

func checkSector(sectors []string) {
	for _, address := range sectors {
		go monitorSector(address)
	}
}

func handleSector(conn net.Conn) {
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	for {
		var message string

		if receiveMessage(decoder, &message) != nil {
			treatSector(conn)
			return
		}

		if message == "PING" {
			if sendMessage(encoder, "PONG") != nil {
				treatSector(conn)
				return
			}
		} else {
			treatSector(conn)
			return
		}
	}
}

func listenSector() {
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Println("Erro ao iniciar servidor: ", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar: ", err)
			continue
		}

		go handleSector(conn)
	}
}

func main() {
	if len(os.Args) < 5 {
		return
	}

	go listenSector()
	go checkSector(os.Args)
}
