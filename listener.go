package ws_chat

import (
	"github.com/gobwas/ws"
	"io"
	"log"
	"net"
	"sync"
)

var wsListeners = make(map[string]net.Listener)

func read(conn net.Conn, w io.Writer, header ws.Header, wg *sync.WaitGroup) {
	defer wg.Done()
	payloadForConn := make([]byte, 400)

	for {

		payload := payloadForConn[:header.Length]

		_, err := conn.Read(payload)
		if err != nil {
			log.Printf("Got error while reading the connction: %s\n", err.Error())
			return
		}

		if header.OpCode == ws.OpClose {
			return
		}

		if header.Masked {
			ws.Cipher(payload, header.Mask, 0)
		}

		_, err = w.Write(payload)
		if err != nil {
			log.Printf("Got error while writeing to writer: %s\n", err.Error())
			return
		}

		header, err = ws.ReadHeader(conn)
		if err != nil {
			log.Printf("Got error while reading header: %s\n", err.Error())
			return
		}
	}
}

func write(conn net.Conn, r io.Reader, header ws.Header, wg *sync.WaitGroup) {
	defer wg.Done()
	payloadForConn := make([]byte, 400)
	header.Masked = false
	for {

		n, err := r.Read(payloadForConn)
		if err != nil {
			log.Printf("Got error while reading from server reader: %s\n", err.Error())
			return
		}

		header.Length = int64(n)

		if err := ws.WriteHeader(conn, header); err != nil {
			log.Printf("Got error while writing header: %s\n", err.Error())
			return
		}

		if _, err := conn.Write(payloadForConn[:n]); err != nil {
			log.Printf("Got error while writing the connction: %s\n", err.Error())
			return
		}
	}
}


func Listen(network, addr string) error {
	lnKey := network + "/" + addr

	//TODO: If listener already exist

	//If listener does not exists
	ln, err := net.Listen(network, addr)
	if err != nil {
		return err
	}

	wsListeners[lnKey] = ln

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		_, err = ws.Upgrade(conn)
		if err != nil {
			return err
		}

		go func() {
			defer conn.Close()

			r, w := io.Pipe()
			var wg sync.WaitGroup

			header, err := ws.ReadHeader(conn)
			if err != nil {
				log.Printf("Got error while reading header: %s\n", err.Error())
				return
			}

			wg.Add(1)
			go read(conn, w, header, &wg)
			wg.Add(1)
			go write(conn, r, header, &wg)


			wg.Wait()
			r.Close()
			w.Close()
			return
		}()
	}
}