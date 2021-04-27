package ws_chat

import (
	"errors"
	"github.com/gobwas/ws"
	"io"
	"log"
	"net"
	"sync"
)
const (
	BinaryType = iota + 1
	StringType

	MaxPayloadSize = 10 << 24 //10 megs
)

//var wsListeners = make(map[string]net.Listener)

var payloadPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 400)
	},
}

type WsPayload []byte

func (wsp WsPayload) Bytes() []byte {return wsp}
func (wsp WsPayload) String() string {return string(wsp)}

func (wsp *WsPayload) ReadFrom(conn io.Reader) (int64, error) {
	frame, err := ws.ReadFrame(conn)
	if err != nil {
		log.Printf("Got error while reading header: %s\n", err.Error())
		return 0, err
	}

	if frame.Header.OpCode.IsControl() {
		//log.Printf("Client close the connection with: %d\n", frame.Header.OpCode)
		return 0, io.EOF
	}

	if frame.Header.Length > MaxPayloadSize {
		return 0, errors.New("maximum payload size exceeded")
	}

	*wsp = frame.Payload

	if frame.Header.Masked {
		ws.Cipher(*wsp, frame.Header.Mask, 0)
	}

	return int64(len(*wsp)), nil
}

func (wsp WsPayload) WriteTo(conn io.Writer) (int64, error) {
	//if err := ws.WriteHeader(conn, ws.Header{
	//	Fin:    true,
	//	Rsv:    0,
	//	OpCode: ws.OpText,
	//	Masked: false,
	//	Mask:   [4]byte{},
	//	Length: int64(len(wsp)),
	//}); err != nil {
	//	log.Printf("Got error while writing header: %s\n", err.Error())
	//	return 0, err
	//}
	if err := ws.WriteFrame(conn, ws.NewFrame(ws.OpText, true, wsp)); err != nil {
		log.Printf("Got error while writing the connction: %s\n", err.Error())
		return 0, err
	}

	return int64(len(wsp)), nil
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

	b := ws.NewCloseFrameBody(1000, "Client close the connection")

	header.Length = int64(len(b))
	if err := ws.WriteFrame(conn, ws.NewCloseFrame(b)); err != nil {
		return err
	}

	return ws.WriteHeader(conn, header)
}

type WsConnection struct {
	closeChan chan struct{}
	closeCond *sync.Cond
	conn net.Conn

	in chan WsPayload
}

func NewWsConnection(conn net.Conn) WsConnection {
	return WsConnection{
		closeChan: make(chan struct{}),
		closeCond: sync.NewCond(&sync.Mutex{}),
		conn:      conn,
		in: make(chan WsPayload),
	}
}

func (wc *WsConnection) read() {
	defer func() {
		log.Println("Reader closed")
		wc.closeChan <- struct{}{}
	}()

	var payload WsPayload = payloadPool.Get().([]byte)
	defer payloadPool.Put(payload)
	for {
		n, err := payload.ReadFrom(wc.conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error while reading from connection: %s\n", err.Error())
			}
			return
		}
		wc.in <- payload[:n]
	}
}

func (wc *WsConnection) write() {
	var payload WsPayload = payloadPool.Get().([]byte)
	defer payloadPool.Put(payload)

	for {
		select {
		case <-wc.closeChan:
			log.Println("Writer closed")
			wc.closeCond.Broadcast()
			return
		case b := <- wc.in:
			_, err := b.WriteTo(wc.conn)
			if err != nil {
				if err != io.EOF {
					log.Printf("Error while reading from connection: %s\n", err.Error())
				}
				return
			}
		}
	}
}

func (wc *WsConnection) close() {
	wc.closeCond.L.Lock()
	wc.closeCond.Wait()
	if err := closeWs(wc.conn); err != nil {
		log.Printf("Error while closeing the connection: %s\n", err.Error())
	}

	if err := wc.conn.Close(); err != nil {
		log.Printf("Error while closeing the connection internal: %s\n", err.Error())
	}
	wc.closeCond.L.Unlock()
}

func ListenWS(srv io.ReadWriteCloser, network, addr string) error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	ln, err := net.Listen(network, addr)
	if err != nil {
		return err
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		_, err = ws.Upgrade(conn)
		if err != nil {
			return err
		}

		wsConn := NewWsConnection(conn)
		go wsConn.read()
		go wsConn.write()
		go wsConn.close()
	}
}