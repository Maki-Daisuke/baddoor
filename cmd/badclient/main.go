package main

import (
	"flag"
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
	var password string
	flag.StringVar(&password, "p", "", "admin password")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: badclient [-p <password>] <host:port>")
		os.Exit(1)
	}
	host := flag.Arg(0)

	// Connect to baddoor server
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Display response from server to stdout
	go func() {
		io.Copy(os.Stdout, conn)
	}()

	// Read password from command line arguments or stdin
	if password == "" {
		bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			log.Fatalf("Can't read password: %v", err)
		}
		password = string(bytePassword)
		fmt.Println()
	}

	// パスワードを送信
	_, err = conn.Write([]byte(password + "\n"))
	if err != nil {
		log.Fatalf("Failed to send password: %v", err)
	}

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
	io.Copy(conn, os.Stdin)
}
