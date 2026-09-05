package main

import (
	"fmt"
	"flag"

	"gvpn/client"
	"gvpn/server"
)

func main() {
	fmt.Println("Starting vpn service...")

	mode := flag.String("mode", "server", "Mode to run the program in: \"server\" or \"client\"")
	flag.Parse()

	switch * mode {
		case "server": {
			fmt.Println("Starting server mode...")
			go server.Start()
		}
		case "client": {
			fmt.Println("Starting client mode...")
			go client.Start()
		}
		case "test" : {
			fmt.Println("Starting same machine testing mode...")
			go client.Start()
			go server.Start()
		}
		default:
			fmt.Println("Invalid mode. Use 'server' or 'client'.")
	}

	for {}
}