package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

func main() {
	// Get host from command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: badclient <host:port>")
		os.Exit(1)
	}
	host := os.Args[1]

	// Connect to baddoor server
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Get terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Failed to set terminal to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				fmt.Println("Exiting...")
				conn.Close()
				os.Exit(0)
			}
		}
	}()

	// Read from stdin and send to server
	go func() {
		io.Copy(conn, os.Stdin)
		conn.Close()
	}()

	// Display response from server to stdout
	io.Copy(os.Stdout, conn)
}
