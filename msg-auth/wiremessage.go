package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
)

// NewWireMessage creates a new message that may be sent over the wire, and is verifiable at the receiver's end
func NewWireMessage(message string, h Hasher) *WireMessage {
	return &WireMessage{
		Length:  uint8(len(message)),
		Message: message,
		// As the hasher is pre-seeded with the secret key, the resulting hash will be H(key + message)
		// This does not change the state of the hasher, hence it's safe for concurrent use
		Hash: h.Hash([]byte(message)),
	}
}

// ParseWireMessage takes in a string sent "on-the-wire", and the byte length of the hash digest (Hasher.Length())
// and returns the WireMessage struct if valid
// ParseWireMessage DOES NOT verify the authenticity of the message
func ParseWireMessage(wirestr string, hashlen uint8) (*WireMessage, error) {
	// Parse the hex-encoded uint8 in the beginning describing the length of the plaintext message
	messagelen64, err := strconv.ParseUint(wirestr[:2], 16, 8)
	if err != nil {
		return nil, err
	}
	// Cast the messagelen variable to uint8
	messagelen := uint8(messagelen64)

	// Verify the length of the message. It should be:
	// a) 1 byte * 2 characters/byte for the length header
	// b) {messagelen} amount of characters for the plaintext message string
	// c) Hasher.Length() bytes * 2 characters/byte
	expectedlen := 1*2 + messagelen + hashlen*2
	if len(wirestr) != int(expectedlen) {
		return nil, fmt.Errorf("length of the parsed message ought to be %d, is actually %d", expectedlen, len(wirestr))
	}

	// Decode the hex-encoded hash string into a byte array
	sentHash, err := hex.DecodeString(wirestr[2+messagelen:])
	if err != nil {
		return nil, err
	}

	// Return a WireMessage object
	return &WireMessage{
		Length:  messagelen,
		Message: wirestr[2 : 2+messagelen],
		Hash:    sentHash,
	}, nil
}

// WireMessage represent a message sent over the network, which can be verified through a shared secret by the receiver
type WireMessage struct {
	// Length describes the length of the Message field
	Length uint8
	// Message contains the original message provided by the user
	Message string
	// Hash is the SHA-3-512 digest of the shared secret between the parties, and the message sent
	Hash []byte
}

// String returns the string representing the bytes sent "over the wire" on the internet
func (wm *WireMessage) String() string {
	return fmt.Sprintf("%02x%s%s", wm.Length, wm.Message, hex.EncodeToString(wm.Hash))
}

// Verify returns true if the message can be successfully verified with the same shared secret the given hasher
// is set to use.
func (wm *WireMessage) Verify(hasher Hasher) bool {
	return bytes.Equal(wm.Hash, hasher.Hash([]byte(wm.Message)))
}
