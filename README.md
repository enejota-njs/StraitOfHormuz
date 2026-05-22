<h1 align="center"> 
 Desbloqueio do Estreito de Ormuz
</h1>

---

<h3 align="center"> 
 Infraestrutura Distribuída para Coordenação de Drones Autônomos de Monitoramento Marítimo
</h3>

---

<details>
  <summary><h2> Descrição do Projeto</h2></summary>

O projeto **Monitoramento Distribuído no Estreito de Ormuz** é um sistema distribuído desenvolvido em **Go (Golang)** (com uma interface de visualização em **Python/Pygame**) para coordenar uma **frota compartilhada de drones autônomos** em um ambiente com **comunicação instável**, **alto volume de eventos simultâneos** e possibilidade de **falha de nós**.

A área operacional é dividida em **setores marítimos**, cada um operado por um **broker de setor** (`sector.go`) responsável por receber eventos de **sensores** (`sensor.go`), manter e propagar uma **fila distribuída de requisições**, e interagir com os **drones** (`drone.go`) que executam as missões de atendimento.

A solução foi projetada para atender aos requisitos principais do cenário: **priorizar ocorrências críticas e/ou mais antigas**, garantir que **um mesmo drone nunca seja reservado para mais de uma requisição ao mesmo tempo**, evitar **duplicidade de atendimento** (dois drones para a mesma requisição) e permitir **replanejamento automático** quando um drone falha ou perde conectividade, mantendo o sistema operando **sem servidor central e sem ponto único de falha**.

</details>

---

<details>
  <summary><h2> Contexto e Problema</h2></summary>

Devido à instabilidade no **Estreito de Ormuz**, uma operação multinacional passou a depender de uma infraestrutura tecnológica capaz de **monitorar rotas marítimas** e **acompanhar comboios civis** com segurança operacional. Nesse cenário, a comunicação é **intermitente**, há **múltiplos eventos simultâneos**, e equipamentos podem ser **destruídos** ou **perder conectividade** a qualquer momento.

Para suportar a operação de forma distribuída, a área do estreito é dividida em **setores marítimos**, cada um gerenciado por um **broker de setor**. Sensores (radares, boias, etc.) geram ocorrências dentro desses setores, e uma **frota compartilhada de drones autônomos** deve ser alocada para atender as requisições críticas, como inspeção visual, identificação de obstáculos e replanejamento de tráfego.

O problema central é garantir que o despacho de drones continue correto e consistente mesmo com concorrência e falhas. Em particular, a solução deve:

1. **Priorizar a liberação de drones** para ocorrências **mais críticas** e/ou para o setor que **solicitou primeiro**  
2. **Garantir exclusão mútua distribuída**, de forma que **um mesmo drone nunca seja reservado ou despachado** para mais de uma ocorrência ao mesmo tempo  
3. **Evitar duplicidade de cobertura**, garantindo que **não sejam enviados dois drones** para atender a **mesma requisição/área**  
4. Manter requisições em uma **fila distribuída**, permitindo **replanejamento automático** quando não há drones disponíveis e quando um drone falha, é abatido ou perde conectividade  
5. Operar **sem qualquer servidor central**, evitando **ponto único de falha**: a queda de um broker de setor não deve interromper o funcionamento dos demais setores nem o despacho de drones em outras regiões

</details>

---

<details>
  <summary><h2> Arquitetura e Decisões de Design</h2></summary>

A solução foi implementada sem frameworks de mensageria (ex.: MQTT, Kafka, RabbitMQ). Toda a comunicação foi construída diretamente sobre a arquitetura nativa da Internet utilizando **sockets TCP** e mensagens **JSON**, priorizando simplicidade, portabilidade e facilidade de depuração em ambiente distribuído com Docker.

### Componentes

- **Broker de Setor (`sector.go`)**  
  Responsável por:
  - Receber eventos de **sensores** do seu setor
  - Criar e manter a **fila local de requisições**
  - Propagar requisições para **outros setores** e para **drones**
  - Receber atualizações de estado dos drones (ex.: `ATTENDING`, `DONE`)
  - Detectar falhas de drones e **replanejar** requisições quando necessário

- **Drone (`drone.go`)**  
  Responsável por:
  - Receber e sincronizar a fila de requisições
  - Participar do mecanismo de **coordenação** para selecionar/assumir requisições
  - Coordenar-se com **outros drones via comunicação P2P** para evitar conflitos de alocação
  - Reportar mudanças de estado ao setor (ex.: iniciou atendimento, concluiu missão)
  - Manter publicação periódica do seu estado para observabilidade

- **Sensor (`sensor.go`)**  
  Responsável por:
  - Carregar sua configuração e identificar o setor correspondente pela posição `(x, y)`
  - Gerar eventos de forma **autônoma e aleatória** (simulação de carga)
  - Enviar leituras para o broker do setor responsável

- **Interface / Observabilidade (`interface.go` + `interface.py`)**  
  Responsável por:
  - Receber estados de **setores**, **drones**, **sensores** e **requisições**
  - Persistir snapshots em JSON (`data/interface/*.json`)
  - Exibir uma visualização em tempo real com **Python/Pygame**  
  **Observação:** a Interface é um componente de monitoramento e não participa da coordenação do sistema (não é usada como controle nem como ponto único de decisão)

### Estilo Arquitetural

O sistema segue um estilo **distribuído e descentralizado**, combinando:
- **Brokers distribuídos por setor** (cada setor é autônomo)
- Comunicação **peer-to-peer** entre os principais componentes

Principais canais P2P:

- **Setor ↔ Setor (P2P):** brokers se comunicam diretamente para propagar e sincronizar requisições  
- **Drone ↔ Setor (P2P):** drones recebem requisições/sincronizam fila e enviam atualizações de estado  
- **Drone ↔ Drone (P2P):** drones se comunicam diretamente para coordenar reservas/atendimentos e evitar que o mesmo drone ou a mesma requisição sejam assumidos de forma concorrente  
- **Sensor → Setor:** sensores enviam eventos para o broker responsável pela sua posição

### Tolerância a falhas e replanejamento

O sistema incorpora replanejamento automático para falhas de drones:

- Quando um **drone falha**, perde conectividade ou é abatido, o broker do setor ou outros drones detectam a ausência e **remove o vínculo** entre o drone e a requisição  
- A requisição que estava em estado **`ATTENDING`** volta para **`PENDING`** e **retorna para a fila distribuída**  
- Em seguida, **outro drone disponível** pode assumir automaticamente essa requisição, respeitando as regras de prioridade e ordenação

### Ausência de ponto único de falha

A coordenação operacional (fila, despacho e replanejamento) ocorre entre **setores e drones**, sem um servidor central responsável por decisões globais.

- Se um **broker de setor** falhar, os **demais setores** continuam recebendo eventos dos seus sensores e mantendo o despacho de drones na sua área  
- A Interface existe apenas para visualização; a falha dela não interrompe o despacho, apenas a visualização e persistência de snapshots

</details>

---

<details>
  <summary><h2> Guia de Uso: Executando com Docker</h2></summary>

Atendendo às restrições do projeto, o sistema foi projetado para rodar em **contêineres Docker**, permitindo executar **múltiplas instâncias isoladas** (setores, drones, sensores e interface) de forma padronizada e reproduzível.

O repositório já inclui um `docker-compose.yml` com os serviços abaixo:

- `sector1`, `sector2`, `sector3`, `sector4` (brokers de setor)
- `drone1`, `drone2` (drones)
- `sensor1`, `sensor2`, `sensor3`, `sensor4` (sensores)
- `interface-server` (coletor/servidor da interface em Go)
- `interface-gui` (painel em Python/Pygame)

---

## 1. Construindo as imagens

Para construir todas as imagens:

```bash
docker compose build
```

Se preferir construir por componente:

```bash
docker compose build sector1 sector2 sector3 sector4
docker compose build drone1 drone2
docker compose build sensor1 sensor2 sensor3 sensor4
docker compose build interface-server interface-gui
```

---

## 2. Executando o ecossistema

Para subir tudo:

```bash
docker compose up
```

Para subir apenas alguns serviços:

```bash
docker compose up interface-server interface-gui
docker compose up sector1 sector2 sector3 sector4
docker compose up drone1 drone2
docker compose up sensor1 sensor2 sensor3 sensor4
```

> Dica: se você quiser acompanhar logs separados, suba grupos em terminais diferentes.

---

## 3. Acessando o painel (interface-gui)

Antes de rodar, em Linux, habilite o acesso do Docker ao servidor X:

```bash
xhost +local:
```

Depois suba a interface:

```bash
docker compose up interface-server interface-gui
```

---

## 4. Portas expostas (mapeamento)

O `docker-compose.yml` expõe portas para facilitar testes e execução distribuída:

- **Setores**
  - `sector1`: `5000`, `5001`, `5002`
  - `sector2`: `5003`, `5004`, `5005`
  - `sector3`: `5006`, `5007`, `5008`
  - `sector4`: `5009`, `5010`, `5011`

- **Drones**
  - `drone1`: `5012`, `5013`
  - `drone2`: `5014`, `5015`

- **Interface (Go)**
  - `interface-server`: `9001`, `9002`, `9003`, `9004`

> As portas específicas usadas na comunicação (sensor/setor/drone) são definidas nos arquivos JSON de inicialização em `data/initialization/`.

---

## 5. Execução em máquinas distintas (laboratório)

Para rodar os contêineres em **computadores diferentes**, é necessário **ajustar os endereços (`host:port`)** nos arquivos de inicialização em `data/initialization/` (ex.: `sectors.json`, `drones.json`, `sensors.json`, `interface.json`) para refletir o **IP/hostname real de cada máquina** no laboratório.

- Se um setor estiver em `10.0.0.21`, então `address_for_sensor`, `address_for_sector` e `address_for_drone` desse setor devem apontar para `10.0.0.21:<porta>` (e não para `localhost`)
- O mesmo vale para drones e para a interface: cada componente deve anunciar/consumir um endereço que seja **alcançável pelos outros nós** na rede

---

## 6. Escalabilidade (adicionar quantos setores/drones/sensores quiser)

O ecossistema foi modelado para permitir adicionar **quantos setores, drones e sensores forem necessários**. Para isso:

1. Crie novas entradas nos arquivos JSON de inicialização em `data/initialization/*.json`
2. Adicione novos serviços no `docker-compose.yml` (ex.: `sector5`, `drone3`, `sensor10`, etc.) apontando o comando para o novo ID
3. Garanta que as **portas não colidam** e que os endereços estejam corretos na rede do laboratório

</details>

---

<details>
  <summary><h2> Conclusão</h2></summary>

O desenvolvimento do projeto **Monitoramento Distribuído no Estreito de Ormuz** cumpriu o desafio de construir uma infraestrutura **distribuída, descentralizada e tolerante a falhas** para coordenação de uma frota compartilhada de **drones autônomos**, operando sob **alta concorrência** e **comunicação instável**.

A divisão da área em **setores marítimos** com **brokers de setor independentes**, aliada à comunicação **P2P**, elimina a dependência de um coordenador central e atende ao requisito de **ausência de ponto único de falha**: a queda de um setor não interrompe o funcionamento dos demais nem paralisa o despacho em outras regiões.

Do ponto de vista de consistência, a solução foi desenhada para:
- **priorizar requisições críticas e/ou mais antigas**, garantindo ordenação mesmo sob atrasos
- assegurar que **um mesmo drone nunca seja reservado/dispatchado para mais de uma ocorrência**
- evitar **duplicidade de cobertura**, impedindo que duas unidades atendam a mesma requisição
- manter requisições em **fila distribuída**, com **replanejamento automático** quando não há drones disponíveis ou quando um drone falha, permitindo que **outro drone assuma** a missão

Por fim, a execução em **contêineres Docker** valida os requisitos de emulação realista no laboratório, possibilitando rodar múltiplos setores, drones e sensores de forma isolada e escalável, inclusive em **máquinas distintas** com ajuste de endereçamento.

</details>

---

<details>
  <summary><h2> Contribuidores</h2></summary>

[<img src="https://github.com/enejota-njs.png" width="80" height="80">](https://github.com/enejota-njs)

</details>

---
