package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"meugo/encoding"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

const (
	memBufferSize = 2 * 1024 * 1024 * 1024 // usando 2GB para o buffer
)

var (
	contador          int
	encontrado        bool
	mu                sync.Mutex
	wg                sync.WaitGroup
	ultimaChaveGerada string
	memBuffer         = make([]byte, memBufferSize)
	enderecoDesejado  string // Endereço desejado fornecido pelo usuário
	tamanhoChave      int    // Tamanho da chave em bits fornecido pelo usuário
	ranges            map[int]string
	modoSequencial    bool     // Determina se estamos em modo sequencial ou random
	chaveAtual        *big.Int // Armazena a chave atual no modo sequencial
)

func init() {
	ranges = make(map[int]string)
}

func carregarRangesDoArquivo(nomeDoArquivo string) {
	file, err := os.Open(nomeDoArquivo)
	if err != nil {
		log.Fatalf("Erro ao abrir o arquivo de ranges: %v", err)
	}
	defer file.Close()

	for {
		var bits int
		var rangeStr string
		_, err := fmt.Fscanf(file, "%d %s\n", &bits, &rangeStr)
		if err != nil {
			break
		}
		ranges[bits] = rangeStr
	}
}

func gerarChavePrivada() string {
	rangeStr, existe := ranges[tamanhoChave]
	if !existe {
		log.Fatalf("Tamanho de chave %d não suportado.", tamanhoChave)
	}

	valores := strings.Split(rangeStr, "-")
	minRange := new(big.Int)
	minRange.SetString(valores[0], 16)
	maxRange := new(big.Int)
	maxRange.SetString(valores[1], 16)

	var chaveGerada *big.Int
	for {
		// Gerar um número aleatório dentro do intervalo especificado
		chaveGerada, _ = rand.Int(rand.Reader, new(big.Int).Sub(maxRange, minRange))
		chaveGerada.Add(chaveGerada, minRange)

		// Verifica se a chave gerada está dentro do intervalo permitido
		if chaveGerada.Cmp(minRange) >= 0 && chaveGerada.Cmp(maxRange) <= 0 {
			break
		}
	}

	// Converter a chave gerada para string hexadecimal
	chaveHex := hex.EncodeToString(chaveGerada.Bytes())

	// Adicionar zeros à esquerda para completar 64 caracteres
	if len(chaveHex) < 64 {
		chaveHex = strings.Repeat("0", 64-len(chaveHex)) + chaveHex
	}

	return chaveHex
}

func worker(id int) {
	defer wg.Done()

	for {
		mu.Lock()
		if encontrado {
			mu.Unlock()
			return
		}
		mu.Unlock()

		chave := gerarChavePrivada()
		pubKeyHash := encoding.CreatePublicHash160(chave)
		address := encoding.EncodeAddress(pubKeyHash)

		mu.Lock()
		contador++
		ultimaChaveGerada = chave
		if address == enderecoDesejado {
			fmt.Printf("\n\n|--------------%s----------------|\n", address)
			fmt.Printf("|----------------------ATENÇÃO-PRIVATE-KEY-----------------------|")
			fmt.Printf("\n|%s|\n", chave)
			encontrado = true
			mu.Unlock()
			return
		}
		mu.Unlock()
	}
}

func main() {
	// Carregar os ranges do arquivo
	carregarRangesDoArquivo("ranges.txt")

	// Mensagem de boas-vindas e entrada de modo
	fmt.Print("\n\n\n\n\n\n")
	fmt.Println(`                                              	        			-_~ BEM VINDO ~_-
						________         __  __         __       _______                            __
						|  |  |  |.---.-.|  ||  |.-----.|  |_    |     __|.-----..---.-..----..----.|  |--.
						|  |  |  ||  _  ||  ||  ||  -__||   _|   |__     ||  -__||  _  ||   _||  __||     |
						|________||___._||__||__||_____||____|   |_______||_____||___._||__|  |____||__|__|  ~ 0.3.9v  By:Ch!iNa ~
							
									  -_~ Carteira Puzzle: 66 67 ~_-
	`)

	// Solicitar a quantidade de bits da chave privada
	fmt.Print("\n\n	Input: Digite a quantidade de bits da chave privada: ")
	fmt.Scanln(&tamanhoChave)

	// Verificar se o tamanho está dentro do intervalo permitido
	if _, ok := ranges[tamanhoChave]; !ok {
		fmt.Printf("	Tamanho de chave %d não suportado. Escolha um valor entre 1 e %d.\n", tamanhoChave, len(ranges))
		return
	}

	// Solicitar o endereço Bitcoin desejado após a escolha do modo e tamanho da chave
	fmt.Print("\n\n	Input: Digite o endereço Bitcoin desejado: ")
	fmt.Scanln(&enderecoDesejado)

	// Obter o número de CPUs disponíveis e o nome do processador
	numCPUs := runtime.NumCPU()
	cpuInfo, _ := cpu.Info()
	cpuModelName := "Desconhecido"
	if len(cpuInfo) > 0 {
		cpuModelName = cpuInfo[0].ModelName
	}

	// Exibe as informações do processador e número de threads
	fmt.Printf("\n	Obs: O Seu Computador tem %d threads. (Processador: %s)\n", numCPUs, cpuModelName)

	// Teste inicial com 3 threads para medir a taxa base
	fmt.Println("\n\n	Iniciando teste de desempenho com 3 threads para medir a taxa base...")
	testThreads := 3
	runtime.GOMAXPROCS(testThreads)

	// Tempo de início para o teste
	startTime := time.Now()

	// Inicia goroutines para o teste
	for i := 0; i < testThreads; i++ {
		wg.Add(1)
		go worker(i)
	}

	// Executa o teste por 5 segundos
	time.Sleep(5 * time.Second)
	mu.Lock()
	elapsedTime := time.Since(startTime).Seconds()
	baseKeysPerSec := float64(contador) / elapsedTime
	mu.Unlock()

	// Finaliza o teste
	encontrado = true
	wg.Wait()
	encontrado = false
	contador = 0
	ultimaChaveGerada = ""

	fmt.Printf("\n\n	Teste concluído. Taxa medida com %d threads: %.2f Chaves/seg\n", testThreads, baseKeysPerSec)

	// Calcular o número de threads com base nas porcentagens
	threads15 := max(1, int(float64(numCPUs)*0.15))
	threads25 := max(1, int(float64(numCPUs)*0.25))
	threads50 := max(1, int(float64(numCPUs)*0.50))
	threads75 := max(1, int(float64(numCPUs)*0.75))
	threads90 := max(1, int(float64(numCPUs)*0.90))
	threads100 := numCPUs

	// Função para calcular a temperatura, RPM do cooler e chaves por segundo
	calculateMetrics := func(percentage float64, threads int) (float64, int, int) {
		baseTemp := 40.0
		tempIncreasePer10Percent := 4.0
		temperature := baseTemp + (percentage/10.0)*tempIncreasePer10Percent

		baseRPM := 100
		rpmIncreasePer10Percent := 25
		rpm := baseRPM + int((percentage/10.0)*float64(rpmIncreasePer10Percent))

		// Estimativa baseada na taxa medida
		keysPerSec := int(baseKeysPerSec / float64(testThreads) * float64(threads))

		return temperature, rpm, keysPerSec
	}

	// Calcular as métricas para cada modo
	temp15, rpm15, keys15 := calculateMetrics(15, threads15)
	temp25, rpm25, keys25 := calculateMetrics(25, threads25)
	temp50, rpm50, keys50 := calculateMetrics(50, threads50)
	temp75, rpm75, keys75 := calculateMetrics(75, threads75)
	temp90, rpm90, keys90 := calculateMetrics(90, threads90)
	temp100, rpm100, keys100 := calculateMetrics(100, threads100)

	fmt.Println("\n\n\n						- Random Mode -")
	fmt.Printf("\n\n		Modo 1: Easy   (15%%) - CPU %.1f°C - %d RPM  - %dk Chaves P/seg", temp15, rpm15, keys15/1000)
	fmt.Printf("\n		Modo 2: Seguro (25%%) - CPU %.1f°C - %d RPM  - %dk Chaves P/seg", temp25, rpm25, keys25/1000)
	fmt.Printf("\n		Modo 3: Medium (50%%) - CPU %.1f°C - %d RPM  - %dk Chaves P/seg", temp50, rpm50, keys50/1000)
	fmt.Printf("\n		Modo 4: Hard   (75%%) - CPU %.1f°C - %d RPM  - %dk Chaves P/seg", temp75, rpm75, keys75/1000)
	fmt.Println("\n\n					- - - A partir daqui já está fritando o CPU - - -")
	fmt.Printf("\n\n		Modo 5: Quase 90%% - CPU %.1f°C - %d RPM  - %dk Chaves P/seg", temp90, rpm90, keys90/1000)
	fmt.Printf("\n		Modo 6: Não Estou nem Aí para meu computador quero que ele queime (100%%) - CPU %.1f°C - %d RPM  - %dk Chaves P/seg", temp100, rpm100, keys100/1000)
	fmt.Print("\n\n	Input: Escolha o modo de acordo com o número correspondente: ")

	var escolha int
	fmt.Scanln(&escolha)

	var numThreads int
	switch escolha {
	case 1:
		numThreads = threads15
	case 2:
		numThreads = threads25
	case 3:
		numThreads = threads50
	case 4:
		numThreads = threads75
	case 5:
		numThreads = threads90
	case 6:
		numThreads = threads100
	default:
		fmt.Println("	Escolha inválida. Usando o SECURE MODE...  (25%)")
		numThreads = threads25
	}
	fmt.Printf("\n  		Iniciando modo %d usando %d threads.\n", escolha, numThreads)
	time.Sleep(1 * time.Second)

	runtime.GOMAXPROCS(numThreads)

	// Tempo de início
	startTime = time.Now()

	// Inicia goroutines
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go worker(i)
	}

	go func() {
		for {
			time.Sleep(1 * time.Second)
			mu.Lock()
			if encontrado {
				mu.Unlock()
				break
			}

			// Calcular o tempo decorrido em segundos
			elapsedTime := time.Since(startTime).Seconds()

			// Calcular a taxa de geração de chaves por segundo
			keysPerSecond := float64(contador) / elapsedTime

			fmt.Printf("\r 	N° Threads Usados: %d | Chaves Geradas: %d | Velocidade: %.2f Chaves/seg ", numThreads, contador, keysPerSecond)
			mu.Unlock()
		}
	}()

	go func() {
		for {
			time.Sleep(2 * time.Second)
			mu.Lock()
			if encontrado {
				mu.Unlock()
				break
			}
			fmt.Print("Ultima Chave Gerada: ", ultimaChaveGerada, " |")
			mu.Unlock()
		}
	}()

	wg.Wait()

	fmt.Print("\n\n	|--------------------------------------------------by-Luan-BSC---|")
	fmt.Print("\n	|-----------------------China-LOOP-MENU------------------------- |")
	fmt.Printf("\n	|		Threads usados: %d		                 |", numThreads)
	fmt.Print("\n	|		Chaves Analisadas:	", contador)
	fmt.Print("\n	|________________________________________________________________|")
}

// Função auxiliar para garantir que não seja menor que 1 thread
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
