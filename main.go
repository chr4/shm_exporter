package main

import (
	"encoding/binary"
	"log"
	"net"
)

const (
	defaultMulticastAddress = "239.12.255.254:9522"
	maxDatagramSize         = 8192
)

func main() {
	listen(defaultMulticastAddress, msgHandler)
}

func msgHandler(src *net.UDPAddr, n int, b []byte) {

	// TODO: Check this
	// b[16:18] == '\x60\x69'
	// b[0:3] == "SMA"

	// TODO: function to convert byte range to float
	totalPowerIn := b[34:36]
	totalPowerOut := b[54:56]
	out := float64(binary.BigEndian.Uint16(totalPowerOut)) * 0.1
	in := float64(binary.BigEndian.Uint16(totalPowerIn)) * 0.1

	log.Printf("out: %f in: %f", out, in)
}

// Listen binds to the UDP address and port given and writes packets received
// from that address to a buffer which is passed to a hander
func listen(address string, handler func(*net.UDPAddr, int, []byte)) {
	// Parse the string address
	addr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		log.Fatal(err)
	}

	// Open up a connection
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		log.Fatal(err)
	}

	conn.SetReadBuffer(maxDatagramSize)

	// Loop forever reading from the socket
	for {
		buffer := make([]byte, maxDatagramSize)
		numBytes, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
		}

		handler(src, numBytes, buffer)
	}
}
