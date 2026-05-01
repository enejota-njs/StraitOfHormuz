package main

import (
	"encoding/json"
	"fmt"
	"net"
)

type Message struct {
	Text string `json:"text"`
}

type Sector struct {
	LonMin float64 `json:"lon_min"`
	LonMax float64 `json:"lon_max"`
}

func sendMessage(conn net.Conn, message Message) error {
	encoder := json.NewEncoder(conn)
	err := encoder.Encode(message)

	if err != nil {
		fmt.Println("Erro ao enviar mensagem: ", err)
		conn.Close()
		return err
	}

	return nil
}

func receiveMessage(conn net.Conn, message Message) error {
	decoder := json.NewDecoder(conn)
	err := decoder.Decode(message)

	if err != nil {
		fmt.Println("Erro ao receber mensagem: ", err)
		conn.Close()
		return err
	}

	return nil
}

func handleSector(conn net.Conn) {

}

func startBroker(port string) {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Println("Erro ao iniciar servidor: ", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao se conectar: ", err)
			return
		}

		go handleSector(conn)
	}
}

func main() {
	go startBroker(":7001")
}
