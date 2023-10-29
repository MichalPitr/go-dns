package main

import (
	"encoding/binary"
	"fmt"
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
	name, idx := decodeDomainName(12)
	b := Body{
		question:   name,
		queryType:  binary.BigEndian.Uint16(body[idx : idx+2]),
		queryClass: binary.BigEndian.Uint16(body[idx+2 : idx+4]),
	}
	return b, idx + 4, nil
}

func decodeDomainName(offset int) (string, int) {
	s := ""
	idx := offset

	for {
		length := int(buffer[idx])
		// length 192 indicates a pointer
		if length == 192 {
			// pointer to a string
			suffix, _ := decodeDomainName(int(buffer[idx+1]))
			s += suffix
			idx += 2
			break
		} else {
			name := buffer[idx+1 : idx+1+length]

			idx += 1 + length

			if buffer[idx] == 0x00 {
				s += string(name)

				idx++
				break
			} else {
				s += string(name) + "."
			}
		}
	}
	return s, idx - offset
}

func decodeAnswer(answer []byte, offset int) (Answer, int, error) {
	name, idx := decodeDomainName(offset)
	qType := binary.BigEndian.Uint16(answer[idx : 2+idx])
	qClass := binary.BigEndian.Uint16(answer[2+idx : 4+idx])
	ttl := binary.BigEndian.Uint32(answer[4+idx : 8+idx])
	rdLength := binary.BigEndian.Uint16(answer[8+idx : 10+idx])
	rData := answer[10+idx : 10+uint16(idx)+rdLength]
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
	return a, idx + int(10+rdLength), nil
}

func printBinary(payload []byte) {
	for i, b := range payload {
		// Print each byte in binary format
		if i%2 == 0 {
			fmt.Printf("%d    ", i/2)
		}
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

var buffer = []byte{}

func main() {
	//             Header            |||                  Body Message            || Qtype, Qclass
	// 0016 0100 0001 0000 0000 0000 |||  0364 6e73 0667 6f6f 676c 6503 636f 6d00 || 0001 0001
	q := Query{
		header: Header{id: 22, flags: 0, qdCount: 1, anCount: 0, nsCount: 0, arCount: 0},
		body:   Body{question: "dns.google.com", queryType: 1, queryClass: 1},
	}
	fmt.Printf("query: %+v\n", q)
	message, err := q.encode()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// fmt.Println(message)
	// for _, b := range message {
	// 	fmt.Printf("%02X ", b)
	// }
	// fmt.Println()

	// sending udp message
	// authoritative root name server
	serverIP := "198.41.0.4:53"
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

	buffer = make([]byte, 1024)
	_, err = conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading from udp:", err)
		return
	}

	// for _, b := range buffer[:n] {
	// 	fmt.Printf("%02X ", b)
	// }
	// fmt.Println()

	responseHeader, err := decodeHeader(buffer[:12])
	if err != nil {
		fmt.Println("Error decoding response header", err)
		return
	}

	fmt.Printf("%b\n", responseHeader.flags)

	fmt.Printf("response header: %+v\n", responseHeader)
	responseBody, size, err := decodeBody(buffer[12:])
	if err != nil {
		fmt.Println("Error decoding response body", err)
		return
	}

	// cache offset of name for decompressing answer section
	names := map[int]string{}
	names[12] = responseBody.question

	fmt.Printf("response body: %+v\n", responseBody)

	// printBinary(buffer[12+size:])
	offset := 12 + size
	for i := 0; i < int(responseHeader.anCount); i++ {
		answer, size, err := decodeAnswer(buffer[offset:], offset)
		if err != nil {
			fmt.Println("Error decoding response answer", err)
			return
		}
		offset += size
		fmt.Printf("response an: %+v\n", answer)
	}

	for i := 0; i < int(responseHeader.nsCount); i++ {
		answer, size, err := decodeAnswer(buffer[offset:], offset)
		if err != nil {
			fmt.Println("Error decoding response answer", err)
			return
		}
		offset += size
		fmt.Printf("response ns: %+v\n", answer)
	}

	for i := 0; i < int(responseHeader.arCount); i++ {
		answer, size, err := decodeAnswer(buffer[offset:], offset)
		if err != nil {
			fmt.Println("Error decoding response answer", err)
			return
		}
		offset += size
		fmt.Printf("response ar: %+v\n", answer)
	}

	s, _ := decodeDomainName(76)
	fmt.Println(s)
	// fmt.Printf("%b\n", buffer[76:100])
	printBinary(buffer)
}
