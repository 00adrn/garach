package server

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

	ifce, err := water.New(config)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	fmt.Printf("Created TUN Interface with name: %s\n", ifce.Name())
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

func listenIfce(conn net.Conn, ifce *water.Interface) {
	fmt.Printf("Now listening on interface '%s'\n", ifce.Name())
	packet := make([]byte, maxPacketSize)

	for {
		n, err := ifce.Read(packet)
		if err != nil {
			fmt.Printf("Error reading packet: %v\n", err)
			return
		}

		fmt.Printf("Read %d bytes from interface '%s'\n", n, ifce.Name())
		if err = writePacket(conn, packet[:n]); err != nil {
			fmt.Printf("Error writing packet to connection: %v\n", err)
			return
		}
	}
}

func makeListener() (net.Listener, error) {
	return net.Listen("tcp", ":"+os.Getenv("SERVER_GVPN_SERVER_PORT"))
}

func listen(conn net.Conn, ifce *water.Interface) {
	for {
		message, err := readPacket(conn)
		if err != nil {
			fmt.Printf("Error reading from connection: %v", err)
			return
		}
		if _, err = ifce.Write(message); err != nil {
			fmt.Printf("Error writing to interface: %v", err)
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
