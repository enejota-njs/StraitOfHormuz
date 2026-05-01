package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

type Drone struct {
	ID     int  `json:"id"`
	IsBusy bool `json:"is_busy"`
	IsOn   bool `json:"is_on"`
}

func listenSector(addressStr string) error {
	clearTerminal()
	reader := bufio.NewReader(os.Stdin)

	address, err := strconv.Atoi(addressStr)
	if err != nil {
		fmt.Println("Porta para setor inválida")
		fmt.Println("Pressione ENTER...")
		reader.ReadString('\n')
		return err
	}

	if address < 7000 || address > 7499 {
		fmt.Println("Selecione uma porta para setor entre 7000 e 7499")
		fmt.Println("Pressione ENTER...")
		reader.ReadString('\n')
		return fmt.Errorf("Porta de setor fora do intervalo")
	}

	listener, err := net.Listen("tcp", "localhost:"+addressStr)
	if err != nil {
		fmt.Println("Erro ao iniciar servidor (setor): ", err)
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

		go handleSector(conn)
	}
}

func listenDrone(addressStr string) error {
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
}

func main() {
	clearTerminal()

	if len(os.Args) < 4 {
		return
	}

	go listenSector(os.Args[1])
	go listenDrone(os.Args[2])

	reader := bufio.NewReader(os.Stdin)
	id, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Println("ID inválido")
		fmt.Println("Pressione ENTER...")
		reader.ReadString('\n')
		return
	}

	drone := Drone{
		ID:     id,
		IsBusy: false,
		IsOn:   true,
	}

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
