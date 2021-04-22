package ws_chat

import "testing"

func TestOpenSocket(t *testing.T) {
	t.Error(Listen("tcp", "localhost:8080"))
}