package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/luxas/socketchat"
)

var nameFlag = flag.String("name", "", "Enter your name")

type cliFunc func(c *Client, args []string) error
type cliHandler struct {
	fn      cliFunc
	numArgs uint8
}

// commands map the command name to the cli handler
var commands = map[string]cliHandler{
	"msg":         cliHandler{msgCmd, 2},
	"new-group":   cliHandler{newGroupCmd, 1},
	"join-group":  cliHandler{joinGroupCmd, 1},
	"leave-group": cliHandler{leaveGroupCmd, 1},
	"quit":        cliHandler{cmdQuit, 0},
	"help":        cliHandler{cmdHelp, 0},
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	flag.Parse()
	name := *nameFlag
	if name == "" {
		return fmt.Errorf("name is empty!")
	}

	log.Printf("Launching client with name %q...\n", name)

	c := NewClient(name)

	if err := c.Connect("unix", socketchat.DefaultServerAddress); err != nil {
		return err
	}
	defer c.Disconnect()

	// Start streaming messages in the background
	c.StartStreaming(os.Stdout)

	// Print help text
	_ = cmdHelp(nil, nil)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ",")
		handler, ok := commands[parts[0]]
		if !ok {
			log.Printf("Invalid command %q", parts[0])
			_ = cmdHelp(nil, nil)
			continue
		}
		args := parts[1:]

		if len(args) != int(handler.numArgs) {
			log.Printf("Invalid number of arguments, expected %d", handler.numArgs)
			_ = cmdHelp(nil, nil)
			continue
		}

		if err := handler.fn(c, args); err != nil {
			log.Printf("Error when executing command %q: %v\n", parts[0], err)
			continue
		}
	}

	if scanner.Err() != nil {
		log.Printf("Scanner experienced errors: %v\n", scanner.Err())
	}

	return nil
}

func msgCmd(c *Client, args []string) error {
	return c.conn.Send(&socketchat.Message{
		Command:  socketchat.CommandMessage,
		Sender:   c.name,
		Receiver: args[0],
		Data:     args[1],
	})
}

func newGroupCmd(c *Client, args []string) error {
	return c.conn.Send(&socketchat.Message{
		Command: socketchat.CommandNewChat,
		Sender:  c.name,
		Data:    args[0],
	})
}

func joinGroupCmd(c *Client, args []string) error {
	return c.conn.Send(&socketchat.Message{
		Command: socketchat.CommandJoinChat,
		Sender:  c.name,
		Data:    args[0],
	})
}

func leaveGroupCmd(c *Client, args []string) error {
	return c.conn.Send(&socketchat.Message{
		Command: socketchat.CommandLeaveChat,
		Sender:  c.name,
		Data:    args[0],
	})
}

func cmdQuit(c *Client, _ []string) error {
	// Notify the server that we're leaving
	if err := c.conn.Send(&socketchat.Message{
		Command: socketchat.CommandLeave,
		Sender:  c.name,
	}); err != nil {
		return err
	}
	// Disconnect from the server
	c.Disconnect()
	os.Exit(0)
	return nil
}

func cmdHelp(_ *Client, _ []string) error {
	fmt.Println(`Usage:
	msg,<receiver>,<message> -- Send a message to a client or group chat
	new-group,<group> -- Create a new group chat
	join-group,<group> -- Join a group chat
	leave-group,<group> -- Leave a group chat
	quit -- Stop this application
	help -- Show this help text`)
	return nil
}

type Client struct {
	name string
	conn *socketchat.Connection
}

func NewClient(name string) *Client {
	return &Client{
		name: name,
	}
}

func (c *Client) Connect(network, address string) error {
	conn, err := net.Dial(network, address)
	if err != nil {
		return err
	}
	c.conn = socketchat.NewConnection(conn)

	err = c.conn.Send(&socketchat.Message{
		Command: socketchat.CommandNewClient,
		Data:    c.name,
	})
	if err != nil {
		return fmt.Errorf("failed to join server: %v", err)
	}

	return nil
}

func (c *Client) Disconnect() {
	log.Println("Client shutting down...")
	c.conn.Close()
}

func (c *Client) StartStreaming(w io.Writer) {
	logger := log.New(w, fmt.Sprintf("client-%s ", c.name), 0)
	go func() {
		for {
			msg, err := c.conn.Receive()
			if err != nil {

				if err == io.EOF {
					log.Printf("Shutting down due to server EOF")
					os.Exit(0)
				}

				logger.Printf("Error when receiving: %v", err)
				continue
			}

			receiver := msg.Receiver
			if receiver == c.name || len(receiver) == 0 {
				receiver = "you"
			}

			logger.Printf("Got message to %s from %s: %s", receiver, msg.Sender, msg.Data)
		}
	}()
}
