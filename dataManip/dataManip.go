package dataManip

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const MaxPacketSize = 65535

func ReadPacket(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	if length > MaxPacketSize {
		return nil, fmt.Errorf("packet too large: %d bytes", length)
	}
	packet := make([]byte, length)
	_, err := io.ReadFull(conn, packet)
	return packet, err
}

func WriteAll(conn net.Conn, data []byte) error {
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

func WritePacket(conn net.Conn, packet []byte) error {
	if len(packet) > MaxPacketSize {
		return fmt.Errorf("packet too large: %d bytes", len(packet))
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(packet)))
	if err := WriteAll(conn, header); err != nil {
		return err
	}
	
	return WriteAll(conn, packet)
}
