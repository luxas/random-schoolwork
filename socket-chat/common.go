package socketchat

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"time"
)

const (
	EnableTLS             = true
	DefaultServerProtocol = "tcp"
	DefaultServerAddress  = "localhost:6443"

	MaxNameByteSize = 32
	MaxDataByteSize = 255
	HeaderSize      = 6

	TimeoutDuration = 1 * time.Minute
)

var (
	MessageStartBytes = []byte{0x00, 0xff}
)

type Command byte

const (
	CommandNewClient Command = iota + 1
	CommandNewChat
	CommandJoinChat
	CommandLeaveChat
	CommandMessage
	CommandLeave
	CommandError
)

type Message struct {
	Command  Command
	Sender   string
	Receiver string
	Data     string
}

var (
	MaxNameSizeError      = fmt.Errorf("size of name exceeded: %d", MaxNameByteSize)
	MaxDataSizeError      = fmt.Errorf("size of message exceeded: %d", MaxDataByteSize)
	SendByteMismatchError = fmt.Errorf("could not send all required bytes")
	ReceiveHeaderError    = fmt.Errorf("could not read header of a message")
)

func NewConnection(c net.Conn) *Connection {
	return &Connection{c, bufio.NewReader(c)}
}

type Connection struct {
	c net.Conn
	r *bufio.Reader
}

func (c *Connection) Send(msg *Message) error {
	//log.Printf("Connection.Send called!")
	if len(msg.Sender) > MaxNameByteSize {
		return MaxNameSizeError
	}
	if len(msg.Receiver) > MaxNameByteSize {
		return MaxNameSizeError
	}
	if len(msg.Data) > MaxDataByteSize {
		return MaxDataSizeError
	}

	data := MessageStartBytes
	data = append(data, []byte{byte(msg.Command), byte(len(msg.Sender)), byte(len(msg.Receiver)), byte(len(msg.Data))}...)
	data = append(data, []byte(msg.Sender)...)
	data = append(data, []byte(msg.Receiver)...)
	data = append(data, []byte(msg.Data)...)
	//log.Println(data)

	n, err := c.c.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("error: %v. expected: %d, sent: %d", err, n, len(data))
	}

	return nil
}

func (c *Connection) Receive() (*Message, error) {
	//log.Printf("Connection.Receive called!")

	headerbuf := make([]byte, HeaderSize)
	n, err := c.r.Read(headerbuf)
	if err != nil {
		return nil, err
	}
	if n != HeaderSize {
		return nil, ReceiveHeaderError
	}
	if !bytes.Equal(headerbuf[:2], MessageStartBytes) {
		return nil, ReceiveHeaderError
	}
	senderSize := headerbuf[3]
	receiverSize := headerbuf[4]
	msgSize := headerbuf[5]
	totalSize := int(senderSize + receiverSize + msgSize)

	databuf := make([]byte, totalSize)
	n2, err := c.r.Read(databuf)
	if err != nil {
		return nil, err
	}
	if n2 != totalSize {
		return nil, ReceiveHeaderError
	}

	return &Message{
		Command:  Command(headerbuf[2]),
		Sender:   string(databuf[:senderSize]),
		Receiver: string(databuf[senderSize : senderSize+receiverSize]),
		Data:     string(databuf[senderSize+receiverSize:]),
	}, nil
}

func (c *Connection) Close() {
	//log.Printf("Closing connection for: %s", c.c.RemoteAddr().String())
	c.c.Close()
}
