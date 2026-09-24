package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"slices"
	"strings"

	"gvpn/cmd"
	"gvpn/dataManip"

	"github.com/joho/godotenv"
	"github.com/songgao/water"
)
var tunMTU int = 1456

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

	select { }
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

func enableIPForwarding() error {
	out, err := cmd.Exec("sudo sysctl -w net.ipv4.ip_forward=1")
	if err != nil {
		fmt.Printf("Error enabling IP forwarding... %s\n", out)
		return err
	}

	return nil
}

func detectOutboundInterface() (string, error) {
	out, err := cmd.Exec("ip route show default")
	if err != nil {
		fmt.Printf("Error detecting outbound interface... %s\n", out)
		return "", err
	}

	tokens := strings.Fields(out)
	outboundIfce := tokens[slices.Index(tokens, "dev")+1]

	fmt.Printf("Detected outbound interface: %s\n", outboundIfce)

	return outboundIfce, nil
}


func configureNAT(tunSubnet, outboundIfce string) error {
	out, err := cmd.Exec(fmt.Sprintf("sudo iptables -t nat -C POSTROUTING -s %s -o %s -j MASQUERADE", tunSubnet, outboundIfce))
	if err == nil {
		return nil
	}

	out, err = cmd.Exec(fmt.Sprintf("sudo iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE", tunSubnet, outboundIfce))
	if err != nil {
		fmt.Printf("Error configuring NAT... %s\n", out)
		return err
	}

	return nil
}

func removeNAT(tunSubnet, outboundIfce string) error {
	out, err := cmd.Exec(fmt.Sprintf("sudo iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE", tunSubnet, outboundIfce))
	if err != nil {
		fmt.Printf("Error removing NAT... %s\n", out)
		return err
	}

	return nil
}

func configureForwardingRules(tunIfce, outboundIfce string) error {
	out, err := cmd.Exec(fmt.Sprintf("sudo iptables -C FORWARD -i %s -o %s -j ACCEPT", tunIfce, outboundIfce))
	if err != nil {
		out, err = cmd.Exec(fmt.Sprintf("sudo iptables -A FORWARD -i %s -o %s -j ACCEPT", tunIfce, outboundIfce))
		if err != nil {
			fmt.Printf("Error configuring forwarding rules... %s\n", out)
			return err
		}
	}

	out, err = cmd.Exec(fmt.Sprintf("sudo iptables -C FORWARD -i %s -o %s -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", outboundIfce, tunIfce))
	if err != nil {
		out, err = cmd.Exec(fmt.Sprintf("sudo iptables -A FORWARD -i %s -o %s -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", outboundIfce, tunIfce))
		if err != nil {
			fmt.Printf("Error configuring forwarding rules... %s\n", out)
			return err
		}
	}

	return nil
}

func removeForwardingRules(tunIfce, outboundIfce string) error {
	out, err := cmd.Exec(fmt.Sprintf("sudo iptables -D FORWARD -i %s -o %s -j ACCEPT", tunIfce, outboundIfce))
	if err != nil {
		fmt.Printf("Error removing forwarding rules... %s\n", out)
		return err
	}

	out, err = cmd.Exec(fmt.Sprintf("sudo iptables -D FORWARD -i %s -o %s -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", outboundIfce, tunIfce))
	if err != nil {
		fmt.Printf("Error removing forwarding rules... %s\n", out)
		return err
	}
	
	return nil
}

func configureTunMTU(ice *water.Interface, mtu int) error {
	out, err := cmd.Exec(fmt.Sprintf("sudo ip link set dev %s mtu %d", ice.Name(), mtu))
	if err != nil {
		fmt.Printf("Error configuring TUN MTU... %s\n", out)
		return err
	}
	
	return nil
}

func createTLSListener(address string, config *tls.Config) (net.Listener, error) {
	listener, err := tls.Listen("tcp", address, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS listener: %v", err)
	}

	return listener, nil
}

func cleanup(tunSubnet, tunIfce, outboundIfce string) {
	removeForwardingRules(tunIfce, outboundIfce)
	removeNAT(tunSubnet, outboundIfce)
}
