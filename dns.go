package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strings"
)

type Query struct {
	header Header
	body   Body
}

type Header struct {
	id      uint16
	flags   uint16
	qdCount uint16
	anCount uint16
	nsCount uint16
	arCount uint16
}

type Body struct {
	question   string
	queryType  uint16
	queryClass uint16
}

func (h Header) encode() ([]byte, error) {
	// fixed size of header at 3 bytes
	serialized := make([]byte, 12)
	binary.BigEndian.PutUint16(serialized[:2], h.id)
	binary.BigEndian.PutUint16(serialized[2:4], h.flags)
	binary.BigEndian.PutUint16(serialized[4:6], h.qdCount)
	binary.BigEndian.PutUint16(serialized[6:8], h.anCount)
	binary.BigEndian.PutUint16(serialized[8:10], h.nsCount)
	binary.BigEndian.PutUint16(serialized[10:12], h.arCount)
	return serialized, nil
}

func (b Body) encode() ([]byte, error) {
	s := strings.Split(b.question, ".")
	length := 0
	for i := range s {
		length += len(s[i])
	}

	// Qtype + Qclass are 4 bytes
	// question is number of bytes for the message + space for counts + termination null
	size := 4 + length + len(s) + 1
	serialized := make([]byte, size)
	idx := 0
	for i := range s {
		serialized[idx] = uint8(len(s[i]))
		for j := 0; j < len(s[i]); j++ {
			serialized[idx+1+j] = s[i][j]
		}
		idx += 1 + len(s[i])
	}
	serialized[idx] = 0

	// Encode the rest of the
	binary.BigEndian.PutUint16(serialized[size-4:size-2], b.queryType)
	binary.BigEndian.PutUint16(serialized[size-2:size], b.queryClass)

	return serialized, nil
}

func (q Query) encode() ([]byte, error) {
	header, err := q.header.encode()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	body, err := q.body.encode()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	combined := append(header, body...)
	return combined, nil
}

func main() {
	//             Header            |||                  Body Message            || Qtype, Qclass
	// 0016 0100 0001 0000 0000 0000 |||  0364 6e73 0667 6f6f 676c 6503 636f 6d00 || 0001 0001
	q := Query{
		header: Header{id: 22, flags: uint16(math.Pow(16, 2)), qdCount: 1, anCount: 0, nsCount: 0, arCount: 0},
		body:   Body{question: "dns.google.com", queryType: 1, queryClass: 1},
	}

	message, err := q.encode()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(message)
	for _, b := range message {
		fmt.Printf("%02X ", b)
	}
	fmt.Println()

	// sending udp message
	serverIP := "8.8.8.8:53"
	conn, err := net.Dial("udp", serverIP)
	if err != nil {
		fmt.Println("Error setting up the UDP connection:", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write(message)
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading from udp:", err)
		return
	}

	for _, b := range buffer[:n] {
		fmt.Printf("%02X ", b)
	}
	fmt.Println()
}
