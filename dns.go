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

type Answer struct {
	body     Body
	ttl      uint32
	rdLength uint16
	rData    []byte
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

func decodeHeader(header []byte) (Header, error) {
	h := Header{
		id:      binary.BigEndian.Uint16(header[:2]),
		flags:   binary.BigEndian.Uint16(header[2:4]),
		qdCount: binary.BigEndian.Uint16(header[4:6]),
		anCount: binary.BigEndian.Uint16(header[6:8]),
		nsCount: binary.BigEndian.Uint16(header[8:10]),
		arCount: binary.BigEndian.Uint16(header[10:12]),
	}
	return h, nil
}

func decodeBody(body []byte) (Body, int, error) {
	s := ""
	idx := 0
	for {
		length := int(body[idx])
		name := body[idx+1 : idx+1+length]

		idx += 1 + length

		if body[idx] == 0x00 {
			s += string(name)
			// step over 0 octet
			idx++
			break
		} else {
			s += string(name) + "."
		}
	}
	fmt.Println("idx:", idx)
	b := Body{
		question:   s,
		queryType:  binary.BigEndian.Uint16(body[idx : idx+2]),
		queryClass: binary.BigEndian.Uint16(body[idx+2 : idx+4]),
	}
	return b, idx + 4, nil
}

func decodeAnswer(answer []byte, names map[int]string) (Answer, int, error) {
	isPointer := (answer[0]>>6)&0b11 == 3
	name := ""
	if isPointer {
		// decode pointer stored in 14 LSBs
		offset := binary.BigEndian.Uint16(answer[:2]) & 0x3FFF
		name = names[int(offset)]
	}
	qType := binary.BigEndian.Uint16(answer[2:4])
	qClass := binary.BigEndian.Uint16(answer[4:6])
	ttl := binary.BigEndian.Uint32(answer[6:10])
	rdLength := binary.BigEndian.Uint16(answer[10:12])
	rData := answer[12 : 12+rdLength]
	a := Answer{
		Body{
			question:   name,
			queryType:  qType,
			queryClass: qClass,
		},
		ttl,
		rdLength,
		rData,
	}
	return a, int(12 + rdLength), nil
}

func printBinary(payload []byte) {
	for i, b := range payload {
		// Print each byte in binary format
		fmt.Printf("%08b ", b)

		// Add a new line every 2 bytes to make it 16 bits per row
		if (i+1)%2 == 0 {
			fmt.Println()
		}
	}

	// Handle the case where the length of byteArray is odd
	if len(payload)%2 != 0 {
		fmt.Println()
	}
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

	responseHeader, err := decodeHeader(buffer[:12])
	if err != nil {
		fmt.Println("Error decoding response header", err)
		return
	}

	fmt.Printf("%+v\n", responseHeader)
	fmt.Printf("%b\n", responseHeader.flags)
	responseBody, size, err := decodeBody(buffer[12:])
	if err != nil {
		fmt.Println("Error decoding response body", err)
		return
	}

	// cache offset of name for decompressing answer section
	names := map[int]string{}
	names[12] = responseBody.question

	fmt.Printf("%+v\n", responseBody)
	fmt.Printf("size: %d\n", size)

	// printBinary(buffer[12+size:])
	offset := 12 + size
	for i := 0; i < int(responseHeader.anCount); i++ {
		answer, size, err := decodeAnswer(buffer[offset:], names)
		if err != nil {
			fmt.Println("Error decoding response answer", err)
			return
		}
		offset += size
		fmt.Printf("%+v\n", answer)
	}
}
