package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/luxas/socketchat"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.Println("Launching server...")
	s := NewServer("unix", socketchat.DefaultServerAddress)
	return s.Serve()
}

type Server struct {
	conns  map[string]*socketchat.Connection
	groups map[string]map[string]bool

	connsMux  *sync.Mutex
	groupsMux *sync.Mutex
	errC      chan error
	lnNetwork string
	lnAddress string
}

func NewServer(network, address string) *Server {
	return &Server{
		conns:     map[string]*socketchat.Connection{},
		groups:    map[string]map[string]bool{},
		connsMux:  &sync.Mutex{},
		groupsMux: &sync.Mutex{},
		lnNetwork: network,
		lnAddress: address,
	}
}

func (s *Server) Serve() error {
	ln, err := net.Listen(s.lnNetwork, s.lnAddress)
	if err != nil {
		return err
	}
	defer ln.Close()

	for {
		select {
		case err := <-s.errC:
			return err
		default:
			c, err := ln.Accept()
			if err != nil {
				return err
			}
			log.Println("Accepted new connection from a client...")

			go s.handleConn(socketchat.NewConnection(c))
		}
	}
}

func (s *Server) handleConn(c *socketchat.Connection) {
	defer c.Close()

	namemsg, err := c.Receive()
	if err != nil || namemsg.Command != socketchat.CommandNewClient {
		log.Printf("Client could not be initialized: %v", err)
		return
	}
	name := namemsg.Data
	s.SetConnection(name, c)

	for {
		msg, err := c.Receive()
		if err != nil {
			if err == io.EOF {
				log.Printf("Shutting down connection to client %s due to EOF", name)
				return
			}

			log.Printf("error reading message: %v", err)
			continue
		}

		log.Printf("Message received from the client: %d %q %q %q", msg.Command, msg.Sender, msg.Receiver, msg.Data)

		switch msg.Command {
		case socketchat.CommandNewChat:
			groupName := msg.Data
			s.groupsMux.Lock()
			_, ok := s.groups[groupName]
			if ok {
				s.groupsMux.Unlock() // TODO: better
				s.returnErrorToClient(c, fmt.Errorf("group %s already exists!", groupName))
				continue
			}
			s.groups[groupName] = map[string]bool{
				msg.Sender: true,
			}
			s.groupsMux.Unlock()

			notifyMsg := fmt.Sprintf("Group %s created by %s!\n", groupName, msg.Sender)
			_ = s.notifyClients(groupName, notifyMsg)
			log.Print(notifyMsg)

		case socketchat.CommandJoinChat:
			groupName := msg.Data
			s.groupsMux.Lock()
			_, ok := s.groups[groupName]
			if !ok {
				s.groupsMux.Unlock() // TODO: better
				s.returnErrorToClient(c, fmt.Errorf("group %s doesn't exist!", groupName))
				continue
			}
			// Register the sender in the group
			s.groups[groupName][msg.Sender] = true
			s.groupsMux.Unlock()

			notifyMsg := fmt.Sprintf("Client %s has joined group %s", msg.Sender, groupName)
			_ = s.notifyClients(groupName, notifyMsg)
			log.Print(notifyMsg)

		case socketchat.CommandLeaveChat:
			groupName := msg.Data
			s.groupsMux.Lock()
			_, ok := s.groups[groupName]
			if !ok {
				s.groupsMux.Unlock() // TODO: better
				s.returnErrorToClient(c, fmt.Errorf("group %s doesn't exist!", groupName))
				continue
			}
			// Remove the sender from the group
			delete(s.groups[groupName], msg.Sender)
			s.groupsMux.Unlock()

			notifyMsg := fmt.Sprintf("Client %s has left group %s", msg.Sender, groupName)
			_ = s.notifyClients(groupName, notifyMsg)
			log.Print(notifyMsg)

		case socketchat.CommandMessage:
			if err := s.sendToClient(msg, nil); err != nil {
				log.Printf("Failed to send message to client: %v", err)
				s.returnErrorToClient(c, err)
				continue
			}

		case socketchat.CommandLeave:
			// If we're asked to close the connection, delete the reference and return
			// TODO: Remove the client from all groups
			s.DeleteConnection(name)
			log.Printf("Client %s has left the server :(", msg.Sender)
			return

		default:
			log.Printf("Couldn't understand the message: %q", msg.Command)
		}
	}
}

func (s *Server) sendToClient(msg *socketchat.Message, overrideReceiver *string) error {
	receiver := msg.Receiver
	if overrideReceiver != nil {
		// In the case of sending messages to groups, we want to keep the group name in msg.Receiver
		receiver = *overrideReceiver
	}

	if receiverc, ok := s.GetConnection(receiver); ok {
		// This message was meant for only one client
		if err := receiverc.Send(msg); err != nil {
			return fmt.Errorf("error forwarding message: %v", err)
		}

		return nil // we're done here
	}

	s.groupsMux.Lock()
	defer s.groupsMux.Unlock()
	members, ok := s.groups[receiver]
	if !ok {
		return fmt.Errorf("client %q not found", receiver)
	}

	for member := range members {
		if err := s.sendToClient(msg, &member); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) notifyClients(clientOrGroup, message string) error {
	return s.sendToClient(&socketchat.Message{
		Command:  socketchat.CommandMessage,
		Receiver: clientOrGroup,
		Sender:   "server",
		Data:     message,
	}, nil)
}

func (s *Server) returnErrorToClient(conn *socketchat.Connection, err error) {
	if err := conn.Send(&socketchat.Message{
		Command: socketchat.CommandError,
		Sender:  "server",
		Data:    err.Error(),
	}); err != nil {
		log.Printf("Failed to return error to client: %v", err)
	}
}

func (s *Server) GetConnection(connID string) (*socketchat.Connection, bool) {
	s.connsMux.Lock()
	defer s.connsMux.Unlock()

	c, ok := s.conns[connID]
	return c, ok
}

func (s *Server) SetConnection(connID string, conn *socketchat.Connection) {
	s.connsMux.Lock()
	defer s.connsMux.Unlock()

	s.conns[connID] = conn
}

func (s *Server) DeleteConnection(connID string) {
	s.connsMux.Lock()
	defer s.connsMux.Unlock()

	delete(s.conns, connID)
}
