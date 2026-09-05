package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"gvpn/cmd"

	"github.com/songgao/water"
	"github.com/joho/godotenv"
)

const maxPacketSize = 65535

func Start() {
	godotenv.Load()

	tun, err := makeTun("192.168.9.10")
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

	ifce, err := water.New(config)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	fmt.Printf("Created TUN Interface with name: %s\n", ifce.Name())
	out, err := cmd.Exec(fmt.Sprintf("sudo ip addr add %s/24 dev %s", ip, ifce.Name()))
	if err != nil {
		fmt.Printf("client tun error adding IP address: %s\n", out)
		return nil, err
	}

	out, err = cmd.Exec(fmt.Sprintf("sudo ip link set dev %s up", ifce.Name()))
	if err != nil {
		fmt.Printf("client tun starting error: %s\n", out)
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
		packet, err := readPacket(conn)
		if err != nil {
			fmt.Printf("client connection read error: %v\n", err)
			return
		}
		if _, err = ifce.Write(packet); err != nil {
			fmt.Printf("client interface write error: %v\n", err)
			return
		}
	}
}

func listenIfce(conn net.Conn, ifce *water.Interface) {
	fmt.Printf("Client interface '%s' now listening\n", ifce.Name())
	packet := make([]byte, maxPacketSize)

	for {
		n, err := ifce.Read(packet)
		if err != nil {
			fmt.Printf("client interface read error: %v\n", err)
			return
		}

		fmt.Printf("Read %d bytes from interface '%s'\n", n, ifce.Name())
		if err = writePacket(conn, packet[:n]); err != nil {
			fmt.Printf("client connection write error: %v", err)
			return
		}
	}
}

func writePacket(conn net.Conn, packet []byte) error {
	if len(packet) > maxPacketSize {
		return fmt.Errorf("packet too large: %d bytes", len(packet))
	}
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(packet)))
	if err := writeAll(conn, header); err != nil {
		return err
	}
	return writeAll(conn, packet)
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		written, err := conn.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func readPacket(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	if length > maxPacketSize {
		return nil, fmt.Errorf("packet too large: %d bytes", length)
	}
	packet := make([]byte, length)
	_, err := io.ReadFull(conn, packet)
	return packet, err
}
