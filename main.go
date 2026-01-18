package main

import (
	"crypto/sha3"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

type Token struct {
	Data         string
	ReceiverHash string
	TTL          int
	SenderID     int
}

func hashID(id int) string {
	h := sha3.New256()
	h.Write([]byte(strconv.Itoa(id)))
	return hex.EncodeToString(h.Sum(nil))
}

type Node struct {
	ID       int
	Hash     string
	Input    <-chan Token
	Output   chan<- Token
	NumNodes int
	Stop     *bool
	Mu       *sync.Mutex
}

func (n *Node) Run(wg *sync.WaitGroup) {
	defer wg.Done()
	for token := range n.Input {
		n.Mu.Lock()
		stopped := *n.Stop
		n.Mu.Unlock()
		if stopped {
			return
		}
		n.processToken(token)
	}
}

func (n *Node) processToken(token Token) {
	if token.ReceiverHash == n.Hash {
		fmt.Printf("[Узел %d] ПОЛУЧЕНО от %d: %s\n", n.ID, token.SenderID, token.Data)
		n.sendNewMessage()
		return
	}

	token.TTL--
	if token.TTL <= 0 {
		fmt.Printf("[Узел %d] TTL истёк: %s\n", n.ID, token.Data)
		n.sendNewMessage()
		return
	}

	fmt.Printf("[Узел %d] Пересылка (TTL=%d): %s\n", n.ID, token.TTL, token.Data)
	n.Output <- token
}

func (n *Node) sendNewMessage() {
	time.Sleep(300 * time.Millisecond)

	n.Mu.Lock()
	stopped := *n.Stop
	n.Mu.Unlock()
	if stopped {
		return
	}

	receiverID := n.ID
	for receiverID == n.ID {
		receiverID = rand.Intn(n.NumNodes)
	}

	msgs := []string{"1", "2", "3", "4", "6"}
	token := Token{
		Data:         string("сообщение " + msgs[rand.Intn(len(msgs))]),
		ReceiverHash: hashID(receiverID),
		TTL:          n.NumNodes + 2,
		SenderID:     n.ID,
	}

	fmt.Printf("[Узел %d] ОТПРАВКА узлу %d: %s (TTL=%d)\n", n.ID, receiverID, token.Data, token.TTL)
	n.Output <- token
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Использование: go run main.go <число_узлов>")
		os.Exit(1)
	}

	numNodes, err := strconv.Atoi(os.Args[1])
	if err != nil || numNodes < 2 {
		fmt.Println("Ошибка: число узлов >= 2")
		os.Exit(1)
	}

	rand.Seed(time.Now().UnixNano())

	channels := make([]chan Token, numNodes)
	for i := 0; i < numNodes; i++ {
		channels[i] = make(chan Token, 1)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	stop := false

	nodes := make([]*Node, numNodes)
	for i := 0; i < numNodes; i++ {
		inputIdx := (i - 1 + numNodes) % numNodes
		nodes[i] = &Node{
			ID:       i,
			Hash:     hashID(i),
			Input:    channels[inputIdx],
			Output:   channels[i],
			NumNodes: numNodes,
			Stop:     &stop,
			Mu:       &mu,
		}
		wg.Add(1)
		go nodes[i].Run(&wg)
	}

	fmt.Printf("Token Ring: %d узлов\n\n", numNodes)

	firstToken := Token{
		Data:         "Стартовое сообщение",
		ReceiverHash: hashID(1),
		TTL:          numNodes + 2,
		SenderID:     -1,
	}
	fmt.Printf("[Главный] ОТПРАВКА узлу 1: %s\n", firstToken.Data)
	channels[numNodes-1] <- firstToken

	time.Sleep(8 * time.Second)

	mu.Lock()
	stop = true
	mu.Unlock()

	for i := 0; i < numNodes; i++ {
		close(channels[i])
	}

	wg.Wait()
	fmt.Println("\nЗавершено")
}

