package server

import (
	"fmt"
	"log"
	"net"
	"os"

	"gvpn/cmd"
	"gvpn/dataManip"

	"github.com/songgao/water"
	"github.com/joho/godotenv"
)

func Start() {
	godotenv.Load(".env")

	listener, err := makeListener()
	if err != nil {
		fmt.Printf("Error creating listener: %v\n", err)
		return
	}
	defer listener.Close()

	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf("Error accepting connection: %v\n", err)
		return
	}
	defer conn.Close()

	tun, err := makeTun("192.168.9.9")
	if err != nil {
		fmt.Printf("Error creating server TUN interface: %v\n", err)
		return
	}
	fmt.Printf("Created TUN Interface with name: %s\n", tun.Name())
	fmt.Printf("Server listening on %s\n", listener.Addr().String())

	go listen(conn, tun)
	go listenIfce(conn, tun)

	for {}
}

func makeTun(ip string) (*water.Interface, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = os.Getenv("SERVER_GVPN_TUN_NAME")
	if config.Name == "" {
		config.Name = "gvpn-server-tun"
	}

	ifce, err := water.New(config)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	out, err := cmd.Exec(fmt.Sprintf("sudo ip addr add %s/24 dev %s", ip, ifce.Name()))
	if err != nil {
		fmt.Printf("Error adding IP address: %s\n", out)
		return nil, err
	}

	out, err = cmd.Exec(fmt.Sprintf("sudo ip link set dev %s up", ifce.Name()))
	if err != nil {
		fmt.Printf("Error setting interface up: %s\n", out)
		return nil, err
	}

	return ifce, nil
}

func listen(conn net.Conn, ifce *water.Interface) {
	for {
		message, err := dataManip.ReadPacket(conn)
		if err != nil {
			fmt.Printf("Error reading from connection: %v", err)
			return
		}

		fmt.Printf("Read %d bytes from TCP connection, writing to TUN...\n", len(message))
		if _, err = ifce.Write(message); err != nil {
			fmt.Printf("Error writing to interface: %v", err)
			return
		}
	}
}

func listenIfce(conn net.Conn, ifce *water.Interface) {
	fmt.Printf("Now listening on interface '%s'\n", ifce.Name())
	packet := make([]byte, dataManip.MaxPacketSize)

	for {
		n, err := ifce.Read(packet)
		if err != nil {
			fmt.Printf("Error reading packet: %v\n", err)
			return
		}

		fmt.Printf("Read %d bytes from '%s', writing to TCP...\n", n, ifce.Name())
		if err = dataManip.WritePacket(conn, packet[:n]); err != nil {
			fmt.Printf("Error writing packet to connection: %v\n", err)
			return
		}
	}
}

func makeListener() (net.Listener, error) {
	return net.Listen("tcp", ":"+os.Getenv("SERVER_GVPN_SERVER_PORT"))
}
