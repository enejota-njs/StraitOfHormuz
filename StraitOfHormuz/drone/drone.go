package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"
)

type Command struct {
	Type       string  `json:"type"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	IsCritical bool    `json:"is_critical"`
}

type Drone struct {
	ID     int     `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	IsBusy bool    `json:"is_busy"`
	IsOn   bool    `json:"is_on"`
}

type Sensor struct {
	Type       string  `json:"type"`
	Longitude  float64 `json:"longitude"`
	IsActive   bool    `json:"is_active"`
	IsCritical bool    `json:"is_critical"`
}

var drone Drone

func receiveCommand(decoder *json.Decoder, command *Command) error {
	err := decoder.Decode(command)

	if err != nil {
		fmt.Println("Erro ao receber comando: ", err)
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

func handleSector(conn net.Conn) {
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	for {
		var command Command
		if receiveCommand(decoder, &command) != nil {
			return
		}

		if command.Type == "REQUEST" {
			if drone.IsBusy || !drone.IsOn {
				sendMessage(encoder, "BUSY")
				continue
			}

			drone.IsBusy = true
			sendMessage(encoder, "SERVING")

			time.Sleep(5 * time.Second)

			drone.IsBusy = false
			sendMessage(encoder, "FINISHED")
		}

		time.Sleep(1 * time.Second)
	}
}

func listenSector() {
	clearTerminal()

	listener, err := net.Listen("tcp", "localhost:7500")
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (setor): ", err)
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

/*func listenDrone(addressStr string) error {
	clearTerminal()
	reader := bufio.NewReader(os.Stdin)

	address, err := strconv.Atoi(addressStr)
	if err != nil {
		fmt.Println("Porta para drone inválida")
		fmt.Println("Pressione ENTER...")
		reader.ReadString('\n')
		return err
	}

	if address < 7500 || address > 7999 {
		fmt.Println("Selecione uma porta para drone entre 7500 e 7999")
		fmt.Println("Pressione ENTER...")
		reader.ReadString('\n')
		return fmt.Errorf("Porta de drone fora do intervalo")
	}

	listener, err := net.Listen("tcp", "localhost:"+addressStr)
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (drone): ", err)
		fmt.Println("Pressione ENTER...")
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar: ", err)
			continue
		}

		go handleDrone(conn)
	}
} */

func main() {
	clearTerminal()

	drone = Drone{
		ID:     1,
		X:      0,
		Y:      0,
		IsBusy: false,
		IsOn:   true,
	}

	go listenSector()

	go func() {
		for {
			fmt.Println(drone)
			time.Sleep(1 * time.Second)
		}
	}()

	select {}
}

func clearTerminal() {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}
