package client

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

	tun, err := makeTun("192.168.9.10")

	fmt.Printf("Created TUN Interface with name: %s\n", tun.Name())
	if err != nil {
		fmt.Printf("Error creating client TUN interface: %v\n", err)
		return
	}

	conn, err := makeConnection()
	if err != nil {
		fmt.Printf("Error creating client connection: %v\n", err)
		return
	}

	defer conn.Close()
	go listen(conn, tun)
	go listenIfce(conn, tun)

	for {}
}

func makeTun(ip string) (*water.Interface, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = os.Getenv("CLIENT_GVPN_TUN_NAME")
	if config.Name == "" {
		config.Name = "gvpn-client-tun"
	}

	ifce, err := water.New(config)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	out, err := cmd.Exec(fmt.Sprintf("sudo ip addr add %s/24 dev %s", ip, ifce.Name()))
	if err != nil {
		fmt.Printf("client TUN error adding IP address: %s\n", out)
		return nil, err
	}

	out, err = cmd.Exec(fmt.Sprintf("sudo ip link set dev %s up", ifce.Name()))
	if err != nil {
		fmt.Printf("client TUN starting error: %s\n", out)
		return nil, err
	}

	return ifce, nil
}

func makeConnection() (net.Conn, error) {
	address := os.Getenv("CLIENT_GVPN_SERVER_ADDR")

	return net.Dial("tcp", address)
}

func listen(conn net.Conn, ifce *water.Interface) {
	for {
		message, err := dataManip.ReadPacket(conn)
		if err != nil {
			fmt.Printf("client connection read error: %v\n", err)
			return
		}

		fmt.Printf("Read %d bytes from TCP connection, writing to TUN...\n", len(message))
		if _, err = ifce.Write(message); err != nil {
			fmt.Printf("client interface write error: %v\n", err)
			return
		}
	}
}

func listenIfce(conn net.Conn, ifce *water.Interface) {
	fmt.Printf("Client interface '%s' now listening\n", ifce.Name())
	packet := make([]byte, dataManip.MaxPacketSize)

	for {
		n, err := ifce.Read(packet)
		if err != nil {
			fmt.Printf("client interface read error: %v\n", err)
			return
		}

		fmt.Printf("Read %d bytes from '%s', writing to TCP...\n", n, ifce.Name())
		if err = dataManip.WritePacket(conn, packet[:n]); err != nil {
			fmt.Printf("client connection write error: %v", err)
			return
		}
	}
}
