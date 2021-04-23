package ws_chat

import (
	"github.com/gobwas/ws"
	"io"
	"log"
	"net"
	"sync"
)

var wsListeners = make(map[string]net.Listener)

var payloadPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 400)
	},
}

func closeWs(conn net.Conn) error {
	header := ws.Header{
		Fin:    true,
		Rsv:    0,
		OpCode: ws.OpClose,
		Masked: false,
		Mask:   [4]byte{},
		Length: 0,
	}
	header.Masked = false

	if err := ws.WriteFrame(conn, ws.NewCloseFrame(ws.NewCloseFrameBody(1000, "Client close the connection"))); err != nil {
		return err
	}

	return ws.WriteHeader(conn, header)
}

func readWs(conn net.Conn, w io.WriteCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	defer w.Close()

	payloadForConn := payloadPool.Get().([]byte)
	defer payloadPool.Put(payloadForConn)

	for {
		header, err := ws.ReadHeader(conn)
		if err != nil {
			log.Printf("Got error while reading header: %s\n", err.Error())
			return
		}

		if header.OpCode == ws.OpClose {
			log.Println("Client close the connection with OpClose")
			return
		}

		payload := payloadForConn[:header.Length]

		_, err = conn.Read(payload)
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
	}
}

func writeWs(conn net.Conn, r io.ReadCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	defer r.Close()
	payloadForConn := payloadPool.Get().([]byte)
	defer payloadPool.Put(payloadForConn)

	header := ws.Header{
		Fin:    true,
		Rsv:    0,
		OpCode: ws.OpText,
		Masked: false,
		Mask:   [4]byte{},
		Length: 0,
	}
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)

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

			log.Printf("connection open: %s\n", conn.RemoteAddr())
			defer log.Printf("connection closed: %s\n", conn.RemoteAddr())

			r, w := io.Pipe()

			var wg sync.WaitGroup

			wg.Add(1)
			go readWs(conn, w, &wg)
			wg.Add(1)
			go writeWs(conn, r, &wg)


			wg.Wait()
			if err := closeWs(conn); err != nil {
				log.Printf("Error while closeing the connection: %s\n", err.Error())
			}
			return
		}()
	}
}