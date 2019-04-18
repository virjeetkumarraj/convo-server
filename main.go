package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Status struct {
	sender *Client
	id     int
}

type BroadcastChat struct {
	sender *Client
	msg    []byte
}

type PersonalChat struct {
	receiver string
	sender   *Client
	msg      []byte
}

type ClientManager struct {
	clients    map[*Client]bool
	broadcast  chan *BroadcastChat
	pc         chan *PersonalChat
	register   chan *Client
	unregister chan *Client
	status     chan *Status
}

type Client struct {
	socket net.Conn
	data   chan []byte
	name   string
}

func (manager *ClientManager) start() {
	go manager.readStatus()
	for {
		select {
		case connection := <-manager.register:
			manager.clients[connection] = true
			fmt.Println(connection.name + " is connected!")
		case connection := <-manager.unregister:
			if _, ok := manager.clients[connection]; ok {
				close(connection.data)
				delete(manager.clients, connection)
				fmt.Println(connection.name + " has disconnected!")
			}
		case b := <-manager.broadcast:
			for connection := range manager.clients {
				if b.sender == connection {
					continue
				}
				select {
				case connection.data <- []byte("/msg " + b.sender.name + " " + string(b.msg)):
					b.sender.sendStatus(manager, 1)
				default:
					b.sender.sendStatus(manager, 2)
					close(connection.data)
					delete(manager.clients, connection)
				}
			}
		case pc := <-manager.pc:
			count := 0
			for connection := range manager.clients {
				if pc.receiver == connection.name {
					count++
					select {
					case connection.data <- []byte("/msg " + pc.sender.name + " " + string(pc.msg)):
						pc.sender.sendStatus(manager, 1)
					default:
						pc.sender.sendStatus(manager, 2)
						close(connection.data)
						delete(manager.clients, connection)
					}
				}
			}
			if count < 1 {
				pc.sender.sendStatus(manager, 2)
			}
		}
	}
}

func (manager *ClientManager) readStatus() {
	for {
		select {
		case status := <-manager.status:
			select {
			case status.sender.data <- []byte("\\x01 " + strconv.Itoa(status.id)):
				fmt.Println("Sending status " + strconv.Itoa(status.id) + " to " + status.sender.name)
			default:
				close(status.sender.data)
				delete(manager.clients, status.sender)
			}
		}
	}
}

func (manager *ClientManager) receive(client *Client) {
	for {
		message := make([]byte, 4096)
		length, err := client.socket.Read(message)
		if err != nil {
			manager.unregister <- client
			client.socket.Close()
			break
		}
		if length > 0 {
			msg := string(message[:length])
			msg = strings.TrimSpace(msg)
			fmt.Println("RECEIVED from " + client.name + ": " + msg)
			args := strings.Split(msg, " ")
			if strings.HasPrefix(msg, "/connect") {
				if len(args) < 3 {
					client.sendStatus(manager, 3)
					continue
				}
				name := strings.Join(args[2:], " ")
				client.name = name
				manager.register <- client
			} else if strings.HasPrefix(msg, "/broadcast") {
				if client.name == "" {
					client.sendStatus(manager, 4)
					continue
				}
				if len(args) < 2 {
					client.sendStatus(manager, 3)
					continue
				}
				manager.broadcast <- &BroadcastChat{client,
					[]byte(strings.TrimPrefix(msg, "/broadcast "))}
			} else if strings.HasPrefix(msg, "/chat") {
				if client.name == "" {
					client.sendStatus(manager, 4)
					continue
				}
				if len(args) < 3 {
					client.sendStatus(manager, 3)
					continue
				}
				receiverName := args[1]
				manager.pc <- &PersonalChat{sender: client, receiver: receiverName,
					msg: []byte(strings.Join(args[2:], " "))}
			} else if msg == "/users" {
				if client.name == "" {
					client.sendStatus(manager, 4)
					continue
				}
				var sb strings.Builder
				for c := range manager.clients {
					if c != client {
						sb.WriteString(c.name + " ")
					}
				}
				select {
				case client.data <- []byte("/users " + sb.String()):
				default:
					close(client.data)
					delete(manager.clients, client)
				}
			} else {
				client.sendStatus(manager, 3)
			}
		}
	}
}

func (manager *ClientManager) send(client *Client) {
	defer client.socket.Close()
	for {
		select {
		case message, ok := <-client.data:
			if !ok {
				return
			}
			fmt.Println("Sending to " + client.name + " : " + string(message))
			client.socket.Write(message)
		}
	}
}

func (client *Client) sendStatus(cm *ClientManager, id int) {
	cm.status <- &Status{sender: client, id: id}
}

func startServerMode() {
	fmt.Println("Starting server...")
	osPort := os.Getenv("PORT")
	if osPort == "" {
		osPort = ":30000"
	} else {
		osPort = ":" + osPort
	}
	listener, error := net.Listen("tcp", osPort)
	if error != nil {
		fmt.Println(error)
		return
	}
	manager := ClientManager{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *BroadcastChat),
		pc:         make(chan *PersonalChat),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		status:     make(chan *Status),
	}
	go manager.start()
	defer listener.Close()
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				fmt.Println(error)
				return
			}
			client := &Client{socket: connection, data: make(chan []byte), name: ""}
			go manager.receive(client)
			go manager.send(client)
		}
	}()
	for {
		message, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		message = strings.TrimSpace(message)
		if message == "exit" {
			break
		}
	}
}

func (client *Client) receive() {
	for {
		message := make([]byte, 4096)
		length, err := client.socket.Read(message)
		if err != nil {
			client.socket.Close()
			break
		}
		if length > 0 {
			msg := string(message[:length])
			fmt.Println("RECEIVED: " + msg)
		}
	}
}

func startClientMode(local bool) {
	fmt.Println("Starting client...")
	host := ""
	if local {
		host = "localhost:30000"
	} else {
		host = "radiant-spire-21595.herokuapp.com:80"
	}
	connection, error := net.Dial("tcp", host)
	if error != nil {
		fmt.Println(error)
		return
	}
	client := &Client{socket: connection}
	defer connection.Close()
	go client.receive()
	for {
		message, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		message = strings.TrimSpace(message)
		if message == "exit" {
			break
		}
		fmt.Println("Sending " + strings.TrimRight(message, "\r\n"))
		connection.Write([]byte(strings.TrimRight(message, "\r\n")))
	}
}

func main() {
	flagMode := flag.String("mode", "server", "start in client or server mode")
	flagLocal := flag.Bool("local", false, "local or not")
	flag.Parse()
	if strings.ToLower(*flagMode) == "server" {
		startServerMode()
	} else {
		startClientMode(*flagLocal)
	}
}
