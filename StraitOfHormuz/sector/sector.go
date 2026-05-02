package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Sensor struct {
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsActive   bool    `json:"is_active"`
	IsCritical bool    `json:"is_critical"`
}

type Drone struct {
	ID        int     `json:"id"`
	Longitude float64 `json:"longitude"`
	IsBusy    bool    `json:"is_busy"`
	IsOn      bool    `json:"is_on"`
}

type Command struct {
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsCritical bool    `json:"is_critical"`
}

type Sector struct {
	XLeft  float64 `json:"x_left"`
	XRight float64 `json:"x_right"`
	YTop   float64 `json:"y_top"`
	YLow   float64 `json:"y_low"`
}

var (
	sector Sector
	drones []string
)

func sendCommand(encoder *json.Encoder, command Command) error {
	err := encoder.Encode(command)

	if err != nil {
		fmt.Println("Erro ao enviar comando: ", err)
		return err
	}

	return nil
}

func sendMessage(encoder *json.Encoder, message string) error {
	err := encoder.Encode(message)

	if err != nil {
		fmt.Println("Erro ao enviar mensagem: ", err)
		return err
	}

	return nil
}

func sendSensor(encoder *json.Encoder, sensor Sensor) error {
	err := encoder.Encode(sensor)

	if err != nil {
		fmt.Println("Erro ao enviar sensor: ", err)
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

// == DRONE

func requestDrone(sensor Sensor) {
	for _, address := range drones {
		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err != nil {
			fmt.Println("Drone indisponível:", address)
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

		if sendCommand(encoder, command) != nil {
			fmt.Println("Drone indisponível:", address)
			conn.Close()
			continue
		}

		var response string

		if receiveMessage(decoder, &response) != nil {
			fmt.Println("Drone indisponível:", address)
			conn.Close()
			continue
		}

		if response == "SERVING" {
			fmt.Println("Drone atendendo solicitação: ", address)
		}

		if receiveMessage(decoder, &response) != nil {
			fmt.Println("Drone indisponível:", address)
			conn.Close()
			continue
		}

		if response == "FINISHED" {
			fmt.Println("Drone finalizou solicitação: ", address)
			conn.Close()
			return
		}

		conn.Close()
	}

	fmt.Println("Nenhum drone aceitou a missão")
}

// == SENSOR

func handleSensor(conn net.Conn) {
	decoder := json.NewDecoder(conn)
	var sensor Sensor

	for {
		if receiveSensor(decoder, &sensor) != nil {
			conn.Close()
			return
		}

		if sensor.IsActive {
			go requestDrone(sensor)
		}
	}
}

func listenSensor() {
	listener, err := net.Listen("tcp", "localhost:9000")
	if err != nil {
		fmt.Println("Erro ao iniciar porta 9000: ", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar com sensor: ", err)
			continue
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
	listener, err := net.Listen("tcp", ":7000")
	if err != nil {
		fmt.Println("Erro ao iniciar porta 7000: ", err)
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

func register() (float64, float64) {
	reader := bufio.NewReader(os.Stdin)

	var left, right float64
	var qtd int

	for {
		fmt.Print("Digite a longitude esquerda: ")
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
		fmt.Print("Digite a longitude direita: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Println("Valor inválido")
			continue
		}

		if val <= left {
			fmt.Println("Deve ser maior que a longitude esquerda")
			continue
		}

		right = val
		break
	}

	for {
		fmt.Print("Quantos drones: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		val, err := strconv.Atoi(input)
		if err != nil || val <= 0 {
			fmt.Println("Número inválido")
			continue
		}

		qtd = val
		break
	}

	for i := 0; i < qtd; i++ {
		for {
			fmt.Printf("Digite o IP do drone %d: ", i+1)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input == "" {
				fmt.Println("Valor inválido")
				continue
			}

			drones = append(drones, input)
			break
		}
	}

	return left, right
}

func main() {
	sector = Sector{
		XLeft:  0,
		XRight: 100,
		YTop:   100,
		YLow:   0,
	}

	go listenSensor()

	select {}
}
