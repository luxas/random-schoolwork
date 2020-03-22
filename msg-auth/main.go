package main

import (
	"flag"
	"fmt"
	"log"
)

// sharedSecret is a flag containing the secret which is shared between both the sender and receiver.
// It is used during the hashing process so that the hash part of the message is: H(secret + message).
var sharedSecret = flag.String("secret", "", "Shared secret")

// hashAlgorithm is a flag for selecting what hashing algorithm to use
var hashAlgorithm = flag.String("algorithm", string(SHA3_512), fmt.Sprintf("The hashing algorithm to use. Options are: %v", SupportedHashAlgorithms()))

// globalHasher is the Hasher instance used by the program at runtime. It uses a certain algorithm, and
// computes the hash digests as needed
var globalHasher Hasher

// main is the entrypoint of the program, it only invokes run()
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Parse the --secret flag
	flag.Parse()

	// Require the shared secret to be given
	if len(*sharedSecret) == 0 {
		return fmt.Errorf("--secret must be set")
	}

	// Validate that the specified algorithm is supported
	algo := HashAlgorithm(*hashAlgorithm)
	if _, ok := hashers[algo]; !ok {
		return fmt.Errorf("hash algorithm %s is not supported; %v are", hashAlgorithm, SupportedHashAlgorithms())
	}

	// Create the hasher object using the specified algorithm
	var err error
	globalHasher, err = NewHasher(algo)
	if err != nil {
		return err
	}
	// Write the shared secret into the hasher as the prefix for all successive .Hash() calls
	globalHasher.Write([]byte(*sharedSecret))

	// Provide two commands for the CLI-based "user-interface", hash and verify, both handled by
	// the referenced Hash() and Verify() functions below
	commands := CLIHandlers{
		"hash":   CLIHandler(Hash, []string{"message"}, "Hash the message that should be transferred to the receiver"),
		"verify": CLIHandler(Verify, []string{"message-on-the-wire"}, "Verify if the message received may be trusted"),
	}

	// Start the listen/command loop for the user
	HandleCommandLoop(commands)
	return nil
}

// Hash takes in a message from the user, and computes the message to be sent over the wire to the receiver
func Hash(args []string) error {
	message := args[0]

	// Create a new WireMessage object for the given message, and hasher, which knows the shared secret
	wm := NewWireMessage(message, globalHasher)

	// Print the string-format of this message-over-the-wire
	printf("Message to send:\n")
	printf("%s\n", wm.String())
	return nil
}

// Verify checks if a given string-encoded message over the wire a) is valid, b) can be trusted
func Verify(args []string) error {
	wiremessage := args[0]

	// Parse the message over the wire into the struct, which is easy to use
	wm, err := ParseWireMessage(wiremessage, globalHasher.Size())
	if err != nil {
		return err
	}

	// Verify the authenticity of the message using the hasher which knows the shared secret
	if wm.Verify(globalHasher) {
		printf("Message verified! You can trust this message\n")
	} else {
		printf("Message has been tampered with! Don't trust this message!!\n")
	}
	return nil
}
