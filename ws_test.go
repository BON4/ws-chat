package ws_chat

import (
	"os"
	"testing"
)

func TestOpenSocket(t *testing.T) {
	//t.Error(ListenWSHTTP("localhost:8080"))
	t.Error(ListenWS(os.Stdout, "tcp", "localhost:8080"))
}