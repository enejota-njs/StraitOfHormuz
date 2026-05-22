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

// Representa um Drone e seus dados de comunicação e estado
type Drone struct {
	AddressForDrone  string  `json:"address_for_drone"`  // Endereço para comunicação entre Drones
	AddressForSector string  `json:"address_for_sector"` // Endereço para comunicação com Setores
	ID               int     `json:"id"`                 // Identificador do Drone
	IsBusy           bool    `json:"is_busy"`            // Indica se o Drone está ocupado
	IsOn             bool    `json:"is_on"`              // Indica se o Drone está ligado
	X                float64 `json:"x"`                  // Coordenada X
	Y                float64 `json:"y"`                  // Coordenada Y
}

// Estrutura utilizada como Mensagem de comunicação entre processos
type Message struct {
	Clock    int       `json:"clock"`    // Relógio lógico da Mensagem
	Drone    Drone     `json:"drone"`    // Dados de Drone
	Request  Request   `json:"request"`  // Requisição individual
	Requests []Request `json:"requests"` // Lista de Requisições
	Text     string    `json:"text"`     // Tipo da Mensagem
}

// Representa uma Requisição gerada e processada no sistema
type Request struct {
	AttendingDroneID int     `json:"attending_drone_id"` // Identificador do Drone que está atendendo
	Clock            int     `json:"clock"`              // Relógio lógico associado à Requisição
	ID               int     `json:"origin_id"`          // Identificador da Requisição na origem
	IsCritical       bool    `json:"is_critical"`        // Indica se a Requisição é Crítica
	SectorID         int     `json:"sector_id"`          // Identificador do Setor de origem
	Status           string  `json:"status"`             // Estado atual da Requisição
	X                float64 `json:"x"`                  // Coordenada X
	Y                float64 `json:"y"`                  // Coordenada Y
}

// Representa um Setor do mapa e suas portas de comunicação
type Sector struct {
	AddressForDrone  string  `json:"address_for_drone"`  // Endereço para comunicação com Drones
	AddressForSector string  `json:"address_for_sector"` // Endereço para comunicação entre Setores
	AddressForSensor string  `json:"address_for_sensor"` // Endereço para comunicação com Sensores
	Bottom           float64 `json:"bottom"`             // Limite inferior
	ID               int     `json:"id"`                 // Identificador do Setor
	Left             float64 `json:"left"`               // Limite esquerdo
	Right            float64 `json:"right"`              // Limite direito
	Top              float64 `json:"top"`                // Limite superior
}

// Representa um Sensor e suas características e estado de ativação
type Sensor struct {
	ID         int     `json:"id"`          // Identificador do Sensor
	IsActive   bool    `json:"is_active"`   // Indica se o Sensor está ativo
	IsCritical bool    `json:"is_critical"` // Indica se o Sensor gera Requisição Crítica
	Type       string  `json:"type"`        // Tipo do Sensor
	X          float64 `json:"x"`           // Coordenada X
	Y          float64 `json:"y"`           // Coordenada Y
}

// Variáveis globais utilizadas para manter o estado local do processo
var (
	clock     int        // Relógio lógico local
	drones    []Drone    // Lista de Drones conhecidos
	mu        sync.Mutex // Exclusão mútua para acesso concorrente ao estado
	requestID int        // Contador local para geração de identificadores de Requisição
	requests  []Request  // Fila/Lista local de Requisições
	sector    Sector     // Setor atual
	sectors   []Sector   // Lista de Setores conhecidos
)

// == CLOCK

// incrementClock Incrementa o Relógio lógico local e retorna o novo valor
func incrementClock() int {
	clock++
	return clock
}

// updateClock Atualiza o Relógio local com base no valor recebido e incrementa para registrar o evento atual
func updateClock(receivedClock int) int {
	if receivedClock > clock {
		clock = receivedClock
	}

	incrementClock()

	return clock
}

// == REQUEST

// addRequestToQueue Adiciona uma Requisição na fila local, evitando duplicatas e mantendo ordenação por prioridade e desempates
func addRequestToQueue(request Request) {
	// Evita duplicidade usando (SectorID, ID) como chave
	for _, r := range requests {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			return
		}
	}

	index := len(requests) // Inserção padrão no final

	for i, r := range requests {
		// Prioriza Requisições Críticas
		if request.IsCritical && !r.IsCritical {
			index = i
			break
		}

		// Mantém Críticas na frente de não Críticas
		if !request.IsCritical && r.IsCritical {
			continue
		}

		// Ordena por Clock para preservar causalidade
		if request.Clock < r.Clock {
			index = i
			break
		}

		// Desempate por Setor
		if request.Clock == r.Clock && request.SectorID < r.SectorID {
			index = i
			break
		}

		// Desempate final por ID da Requisição
		if request.Clock == r.Clock &&
			request.SectorID == r.SectorID &&
			request.ID < r.ID {
			index = i
			break
		}
	}

	// Insere na posição calculada mantendo o restante da fila
	requests = append(requests[:index], append([]Request{request}, requests[index:]...)...)

	fmt.Println("\nFila atual:\n")
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

// sendRequest Cria uma Requisição a partir de um Sensor, registra localmente e propaga para Setores e Drones
func sendRequest(sensor Sensor) {
	mu.Lock()

	clockValue := incrementClock()
	requestID++

	// Define o conteúdo da Requisição com base no Sensor e no estado do Setor atual
	request := Request{
		SectorID:   sector.ID,
		ID:         requestID,
		Status:     "PENDING",
		X:          sensor.X,
		Y:          sensor.Y,
		IsCritical: sensor.IsCritical,
		Clock:      clockValue,
	}

	fmt.Printf("\nNova requisição criada -> SectorID: %d | RequestID: %d | X: %.2f | Y: %.2f | Critical: %t | Clock: %d\n", request.SectorID, request.ID, request.X, request.Y, request.IsCritical, request.Clock)

	addRequestToQueue(request)

	// Atualiza a Interface para visualização do estado
	go sendRequestToInterface("data/initialization/interface.json", request)

	message := Message{
		Text:    "REQUEST",
		Request: request,
		Clock:   clockValue,
	}

	// Copia listas para evitar inconsistência durante iterações sem lock
	currentSectors := append([]Sector(nil), sectors...)
	currentDrones := append([]Drone(nil), drones...)

	mu.Unlock()

	// Propaga a Requisição para os demais Setores
	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("\nSetor indisponível: ID ", s.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message

		// Resposta é usada para sincronizar o Relógio lógico
		if err = decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "QUEUED" {
			_ = conn.Close()
		}
	}

	// Propaga a Requisição para os Drones, permitindo que eles iniciem o despacho
	for _, d := range currentDrones {
		conn, err := net.DialTimeout("tcp", d.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("\nDrone indisponível: ID ", d.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message

		// Resposta é usada para sincronizar o Relógio lógico
		if err = decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "QUEUED" {
			_ = conn.Close()
		}
	}
}

// markRequestAsAttending Marca a Requisição como em atendimento e registra o Drone responsável
func markRequestAsAttending(request Request, attendingDrone Drone) {
	fmt.Printf("\nDrone aceitou requisição -> DroneID: %d | SectorID: %d | RequestID: %d\n", attendingDrone.ID, request.SectorID, request.ID)

	// Atualiza a fila local para refletir o início do atendimento
	for i := range requests {
		if requests[i].SectorID == request.SectorID && requests[i].ID == request.ID {
			requests[i].Status = "ATTENDING"
			requests[i].AttendingDroneID = attendingDrone.ID
			break
		}
	}
}

// removeRequestDone Remove a Requisição concluída da fila local do Setor
func removeRequestDone(request Request) {
	fmt.Printf("\nRequisição finalizada -> SectorID: %d | RequestID: %d\n", request.SectorID, request.ID)

	var filtered []Request

	for _, r := range requests {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			continue
		}

		filtered = append(filtered, r)
	}

	requests = filtered
}

// == DRONE

// handleDroneCrash Atualiza o estado do Drone como indisponível e reabre Requisições que estavam em atendimento por ele
func handleDroneCrash(crashedDroneID int) {
	mu.Lock()
	defer mu.Unlock()

	for i := range drones {
		// Marca o Drone como desligado e livre para evitar seleção futura
		if drones[i].ID == crashedDroneID {
			drones[i].IsOn = false
			drones[i].IsBusy = false
			break
		}
	}

	for i := range requests {
		// Reverte para PENDING quando a Requisição ficou sem Drone responsável
		if requests[i].Status == "ATTENDING" &&
			requests[i].AttendingDroneID == crashedDroneID {

			requests[i].Status = "PENDING"
			requests[i].AttendingDroneID = 0

			pendingRequest := requests[i]

			// Atualiza a Interface para refletir a reabertura da Requisição
			go sendRequestToInterface(
				"data/initialization/interface.json",
				pendingRequest,
			)
		}
	}
}

// monitorDrones Verifica periodicamente se os Drones respondem e aciona tratamento de falha quando necessário
func monitorDrones() {
	for {
		mu.Lock()
		currentDrones := append([]Drone(nil), drones...)
		mu.Unlock()

		for _, d := range currentDrones {
			// Ignora Drones já marcados como desligados
			if !d.IsOn {
				continue
			}

			conn, err := net.DialTimeout("tcp", d.AddressForDrone, 2*time.Second)
			if err != nil {
				fmt.Println("Drone não respondeu:", d.ID)
				handleDroneCrash(d.ID)
			} else {
				_ = conn.Close()
			}
		}

		time.Sleep(3 * time.Second)
	}
}

// handleDrone Processa mensagens vindas de Drones e mantém a fila local consistente
func handleDrone(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if decoder.Decode(&message) != nil {
		return
	}

	switch message.Text {
	case "ATTENDING":
		// Registra que a Requisição entrou em atendimento e sincroniza o Relógio lógico
		mu.Lock()
		currentClock := updateClock(message.Clock)
		markRequestAsAttending(message.Request, message.Drone)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})

	case "DONE":
		// Remove a Requisição concluída e sincroniza o Relógio lógico
		mu.Lock()
		currentClock := updateClock(message.Clock)
		removeRequestDone(message.Request)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "REMOVED",
			Clock: currentClock,
		})

	case "SYNC_REQUESTS":
		// Responde com a fila atual para permitir sincronização entre componentes
		mu.Lock()
		currentClock := updateClock(message.Clock)
		currentRequests := append([]Request(nil), requests...)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:     "REQUESTS_SYNCED",
			Requests: currentRequests,
			Clock:    currentClock,
		})

	case "PENDING":
		// Reabre a Requisição no estado local e remove o vínculo com o Drone anterior
		mu.Lock()

		currentClock := updateClock(message.Clock)

		for i := range requests {
			if requests[i].SectorID == message.Request.SectorID &&
				requests[i].ID == message.Request.ID {

				requests[i].Status = "PENDING"
				requests[i].AttendingDroneID = 0

				break
			}
		}

		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})
	}
}

// listenDrone Inicia o servidor TCP do Setor para receber mensagens enviadas por Drones
func listenDrone() {
	_, port, _ := net.SplitHostPort(sector.AddressForDrone)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos drones: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (drone)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleDrone(conn)
	}
}

// == SENSOR

// handleSensor Recebe leituras do Sensor via conexão TCP e dispara Requisições quando o Sensor está ativo
func handleSensor(conn net.Conn) {
	decoder := json.NewDecoder(conn)

	var sensor Sensor

	for {
		// Mantém leitura contínua até a conexão falhar
		if err := decoder.Decode(&sensor); err != nil {
			_ = conn.Close()
			return
		}

		// Apenas Sensores ativos geram Requisições
		if sensor.IsActive {
			go sendRequest(sensor)
		}
	}
}

// listenSensor Inicia o servidor TCP do Setor para receber eventos enviados por Sensores
func listenSensor() {
	_, port, _ := net.SplitHostPort(sector.AddressForSensor)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos sensores: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (sensor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleSensor(conn)
	}
}

// == SECTOR

// handleSector Processa mensagens vindas de outros Setores e enfileira Requisições recebidas
func handleSector(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if decoder.Decode(&message) != nil {
		return
	}

	switch message.Text {
	case "REQUEST":
		// Registra recebimento e garante ordem pelo Relógio lógico
		fmt.Printf("\nRequisição recebida -> SectorID: %d | RequestID: %d | X: %.2f | Y: %.2f | Critical: %t | Clock: %d\n", message.Request.SectorID, message.Request.ID, message.Request.X, message.Request.Y, message.Request.IsCritical, message.Request.Clock)

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

// listenSectors Inicia o servidor TCP do Setor para receber mensagens de outros Setores
func listenSectors() {
	_, port, _ := net.SplitHostPort(sector.AddressForSector)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos setores: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (setor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleSector(conn)
	}
}

// == LOAD DATA

// loadSectors Carrega a configuração de Setores, seleciona o Setor atual e mantém os demais como Setores conhecidos
func loadSectors(path string, myID int) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de setores: ", err)
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Sector
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	var filtered []Sector

	for _, s := range config {
		// Separa o Setor deste processo do restante da lista
		if s.ID == myID {
			sector = s
			continue
		}

		filtered = append(filtered, s)
	}

	sectors = filtered

	return nil
}

// loadDrones Carrega a lista de Drones conhecidos a partir de um arquivo JSON
func loadDrones(path string) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de drones: ", err)
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

// SAVE DATA

// sendRequestToInterface Envia uma Requisição para o arquivo de Requisições da Interface
func sendRequestToInterface(path string, request Request) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de interface: ", err)
		return
	}
	defer file.Close()

	var config []struct {
		Sectors  string `json:"sectors"`
		Drones   string `json:"drones"`
		Sensors  string `json:"sensors"`
		Requests string `json:"requests"`
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return
	}

	conn, err := net.DialTimeout("tcp", config[0].Requests, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao conectar com interface para enviar requisição: ", err)
		return
	}
	defer conn.Close()

	_ = json.NewEncoder(conn).Encode(request)
}

// sendSectorToInterface Envia o estado do Setor atual para a Interface, com tentativa até conseguir conexão
func sendSectorToInterface(path string) {
	mu.Lock()
	currentSector := sector
	mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de interface: ", err)
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
		return
	}

	for {
		// Mantém tentativa para não iniciar sem registrar na Interface
		conn, err := net.DialTimeout("tcp", config[0].Sectors, 2*time.Second)
		if err != nil {
			fmt.Println("Interface indisponível, tentando novamente...")
			time.Sleep(1 * time.Second)
			continue
		}

		if err = json.NewEncoder(conn).Encode(currentSector); err != nil {
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}
}

// MAIN

// main Inicializa o Setor pelo ID informado, carrega configurações e inicia as rotinas de rede e monitoramento
func main() {
	if len(os.Args) < 2 {
		return
	}

	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Erro no Atoi")
		return
	}

	sectorsPath := "data/initialization/sectors.json"
	dronesPath := "data/initialization/drones.json"
	intefacePath := "data/initialization/interface.json"

	// Carrega Setor atual e lista de Setores remotos
	if err := loadSectors(sectorsPath, id); err != nil {
		fmt.Println("ERRO AO CARREGAR SECTORS:", err)
		return
	}
	// Carrega lista de Drones conhecidos
	if err := loadDrones(dronesPath); err != nil {
		fmt.Println("ERRO AO CARREGAR DRONES:", err)
		return
	}

	// Publica o Setor na Interface para visualização
	go sendSectorToInterface(intefacePath)

	// Inicia servidores e rotinas principais do Setor
	go listenSensor()
	go listenSectors()
	go listenDrone()
	go monitorDrones()

	select {}
}
