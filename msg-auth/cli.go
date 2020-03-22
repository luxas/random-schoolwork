package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type cliFunc func(args []string) error
type cliHandler struct {
	fn          cliFunc
	args        []string
	description string
}

type CLIHandlers map[string]cliHandler

func CLIHandler(fn cliFunc, args []string, desc string) cliHandler {
	return cliHandler{fn, args, desc}
}

func printf(format string, args ...interface{}) {
	fmt.Printf("> "+format, args...)
}

func HandleCommandLoop(cmds CLIHandlers) {
	cmdHelp(cmds)

	// Create a new scanner
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("$ ")
		scanner.Scan()
		if scanner.Err() != nil {
			printf("Scanner experienced errors: %v\n", scanner.Err())
			os.Exit(1)
		}

		parts := strings.Split(scanner.Text(), ",")
		command := parts[0]
		switch command {
		case "quit", "exit":
			os.Exit(0)
		case "help":
			cmdHelp(cmds)
			continue
		default: // just continue below
		}

		handler, ok := cmds[command]
		if !ok {
			printf("Invalid command %q\n", command)
			cmdHelp(cmds)
			continue
		}
		args := parts[1:]

		if len(args) != len(handler.args) {
			printf("Invalid number of arguments, expected %d\n", len(handler.args))
			cmdHelp(cmds)
			continue
		}

		if err := handler.fn(args); err != nil {
			printf("Error when executing command %q: %v\n", parts[0], err)
			continue
		}
	}
}

func cmdHelp(commands CLIHandlers) {
	printf("Usage:\n")
	for cmd, handler := range commands {
		argStr := ""
		for _, arg := range handler.args {
			argStr += "," + arg
		}
		printf("%s%s -- %s\n", cmd, argStr, handler.description)
	}
	printf("help -- Show this help message\n")
	printf("quit -- Quit the program\n")
}
