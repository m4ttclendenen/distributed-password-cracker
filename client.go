package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"crypto/md5"
	"encoding/hex"
)

type Message struct {
	Code	int
	Payload	json.RawMessage
}
type Range struct {
	Start	int
	End		int
}
type User struct {
	ID		int
	Pass	int
}
type Job struct {
	Hash	string
	Range	Range
}
type Password struct {
	Value	string
}
// keeps reading from server
func readServer(conn net.Conn, serverChan chan []byte) {
	ba := make([]byte, 2048)
	for {
		// blocking
		n, err := conn.Read(ba)
		if err != nil {
			fmt.Printf("Error %v", err)
			return
		}
		fmt.Printf("Incoming Message\n")
		serverChan <- ba[:n]
	}
}
// crack the hash
func work(job Job, workChan chan []byte) {
	fmt.Printf("Starting New Job: %s\n", job.Hash)
	for i := job.Range.Start; i <= job.Range.End; i++ {
		iStr := strconv.Itoa(i)
		hasher := md5.New()
		hasher.Write([]byte(iStr))
		possibleHash := hex.EncodeToString(hasher.Sum(nil))
		// compare computed hash to known hash given by job
		if possibleHash == job.Hash {
			password := Password {
				Value: iStr,
			}
			p, err := json.Marshal(password)
			if err != nil {
				fmt.Printf("Error %v", err)
				return
			}
			outMessage := Message {
				Code: 3,
				Payload: p,
			}
			om, err := json.Marshal(outMessage)
			if err != nil {
				fmt.Printf("Error %v", err)
				return
			}
			workChan <- om
			return
		}
	}
	// done with job. we will need another one
	outMessage := Message {
		Code: 1,
		Payload: nil,
	}
	om, err := json.Marshal(outMessage)
	if err != nil {
		fmt.Printf("Some error %v", err)
		return
	}
	workChan <- om
}

func main() {

	// This is the initial part of the process. We dial the connection to the main server
	// and then we make the request for a job.

	// dial connection
	conn, err := net.Dial("udp", "127.0.0.1:1234")
	if err != nil {
		fmt.Printf("Some error %v", err)
		return
	}
	// build marshalled data for initial request
	outMessage := Message {
		Code: 1,
		Payload: nil,
	}
	om, err := json.Marshal(outMessage)
	if err != nil {
		fmt.Printf("Error %v", err)
		return
	}
	// send initial request to server
	fmt.Fprintf(conn, string(om))
	fmt.Printf("\nInitial Request Sent!\nCode: 1\n")
	// channels
	serverChan := make(chan []byte)
	workChan := make(chan []byte)
	// concurrent function call
	// reads messages from server and passes through channel
	go readServer(conn, serverChan)

	/*
		This is where most of the work will be done. All transactions are handled and passed along channels.
		The for loop cycles through channels via select to see if any have active messages coming through.
	*/

	// listen on channels
	for {
		select {
		// server channel
		case sch := <-serverChan:
			fmt.Printf("SERVER : %s\n", sch)
			inMessage := Message{}
			err = json.Unmarshal(sch, &inMessage)
			if err != nil {
				fmt.Printf("Error %v", err)
				return
			}
			if inMessage.Code == 2 {
				job := Job{}
				err = json.Unmarshal(inMessage.Payload, &job)
				if err != nil {
					fmt.Printf("Error %v", err)
					return
				}
				go work(job, workChan)
			}
		case wch := <-workChan:
			inMessage := Message{}
			err = json.Unmarshal(wch, &inMessage)
			if err != nil {
				fmt.Printf("Error %v", err)
				return
			}
			// response - return ranges
			if inMessage.Code == 99 {
				// forward ranges to server
				fmt.Printf("Forward Ranges\n")
			} else if inMessage.Code == 1 {
				fmt.Fprintf(conn, string(wch))
			} else if inMessage.Code == 3 {
				if err != nil {
					fmt.Printf("Error %v", err)
					return
				}
				fmt.Printf("Sending the Password\n")
				fmt.Fprintf(conn, string(wch))
				fmt.Printf("END!")
				return
			}
		default:
			continue
		}
	}
}
