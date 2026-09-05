package main

import (
	"fmt"

	"gvpn/client"
	"gvpn/server"
)

func main() {
	fmt.Println("Starting vpn service...")

	fmt.Println("Starting server...")
	go server.Start()

	fmt.Println("Starting client...")
	go client.Start()

	for {}
}
