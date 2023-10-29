package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

type Stack []string

// Push adds an item to the top of the stack
func (s *Stack) Push(item string) {
	*s = append(*s, item)
}

// Pop removes the item from the top of the stack and returns it
func (s *Stack) Pop() (string, error) {
	if len(*s) == 0 {
		return "", errors.New("pop from empty stack")
	}

	index := len(*s) - 1   // Get the index of the top most element.
	element := (*s)[index] // Index into the slice and obtain the element.
	*s = (*s)[:index]      // Remove it from the stack by slicing it off.
	return element, nil
}

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

type Resource struct {
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

func decodeNSrData(rdata []byte) string {
	s := ""
	idx := 0
	for {
		length := int(rdata[idx])
		// length 192 indicates a pointer
		if length == 192 {
			// pointer to a string in the original response buffer
			suffix, _ := decodeDomainName(int(rdata[idx+1]))
			s += suffix
			idx += 2
			break
		} else {
			name := rdata[idx+1 : idx+1+length]
			idx += 1 + length
			if rdata[idx] == 0x00 {
				s += string(name)
				idx++
				break
			} else {
				s += string(name) + "."
			}
		}
	}
	return s
}

func decodeResource(answer []byte, offset int) (Resource, int, error) {
	name, idx := decodeDomainName(offset)
	qType := binary.BigEndian.Uint16(answer[idx : 2+idx])
	qClass := binary.BigEndian.Uint16(answer[2+idx : 4+idx])
	ttl := binary.BigEndian.Uint32(answer[4+idx : 8+idx])
	rdLength := binary.BigEndian.Uint16(answer[8+idx : 10+idx])

	rData := []byte{}
	if qType == 2 && qClass == 1 {
		rData = []byte(decodeNSrData(answer[10+idx : 10+uint16(idx)+rdLength]))
	} else {
		rData = answer[10+idx : 10+uint16(idx)+rdLength]
	}
	a := Resource{
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

// Global variables to avoid passing vars around
var buffer = []byte{}
var verbose = false

const udpMaxPacketSize = 512
const defaultRootNameServer = "192.203.230.10"

func resolveDomainName(domainName string, nameServer string) (string, error) {
	q := Query{
		header: Header{id: 22, flags: 0, qdCount: 1, anCount: 0, nsCount: 0, arCount: 0},
		body:   Body{question: domainName, queryType: 1, queryClass: 1},
	}

	message, err := q.encode()
	if err != nil {
		return "", err
	}

	// sending udp message
	// authoritative root name server
	serverIP := defaultRootNameServer

	// Keep track of servers already queried to avoid cycles.
	visited := map[string]struct{}{}
	visited[serverIP] = struct{}{}
	stack := Stack{serverIP}

	for len(stack) > 0 {
		ip, err := stack.Pop()
		if err != nil {
			return "", err
		}
		conn, err := net.Dial("udp", fmt.Sprintf("%s:53", ip))
		if err != nil {
			return "", err
		}
		defer conn.Close()

		fmt.Printf("Querying %s for %s\n", ip, domainName)
		_, err = conn.Write(message)
		if err != nil {
			return "", err
		}

		buffer = make([]byte, udpMaxPacketSize)
		_, err = conn.Read(buffer)
		if err != nil {
			return "", err
		}

		responseHeader, err := decodeHeader(buffer[:12])
		if err != nil {
			return "", err
		}
		if responseHeader.id != q.header.id {
			return "", fmt.Errorf("Expected response with id '%d' but got '%d' instead.", q.header.id, responseHeader.id)
		}

		if responseHeader.anCount+responseHeader.nsCount+responseHeader.arCount == 0 {
			return "", fmt.Errorf("No records received from server.")
		}

		responseBody, size, err := decodeBody(buffer[12:])
		if err != nil || responseBody.question != q.body.question {
			return "", err
		}

		offset := 12 + size
		for i := 0; i < int(responseHeader.anCount); i++ {
			answer, _, err := decodeResource(buffer[offset:], offset)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d.%d.%d.%d", answer.rData[0], answer.rData[1], answer.rData[2], answer.rData[3]), nil
		}

		authorityRecords := make([]Resource, 0)
		for i := 0; i < int(responseHeader.nsCount); i++ {
			answer, size, err := decodeResource(buffer[offset:], offset)
			if err != nil {
				return "", err
			}
			authorityRecords = append(authorityRecords, answer)
			offset += size
		}

		additionalRecords := make([]Resource, 0)
		for i := 0; i < int(responseHeader.arCount); i++ {
			answer, size, err := decodeResource(buffer[offset:], offset)
			if err != nil {
				return "", err
			}
			additionalRecords = append(additionalRecords, answer)
			offset += size
		}

		for i := range additionalRecords {
			// We have ipv4 address for server that can help resolve the query
			ar := additionalRecords[i]
			if ar.body.queryType == 1 && ar.body.queryClass == 1 && ar.rdLength == 4 {
				newIP := fmt.Sprintf("%d.%d.%d.%d", ar.rData[0], ar.rData[1], ar.rData[2], ar.rData[3])
				if _, exists := visited[newIP]; !exists {
					stack.Push(newIP)
					visited[newIP] = struct{}{}
				}
			}
		}

		// Resolve name server's ip
		if len(stack) == 0 {
			nameServer, err := resolveDomainName(string(authorityRecords[0].rData), serverIP)
			if err != nil {
				return "", err
			}
			stack.Push(nameServer)
		}

	}
	return "", fmt.Errorf("Failed to resolve this domain name.")
}

func main() {
	args := os.Args[1:]
	if len(args) != 1 {
		fmt.Println("Usage: ./dns domain")
		os.Exit(0)
	}

	domain, err := resolveDomainName(args[0], "192.203.230.10")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println(domain)
	return
}
