package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFragmentedConcurrentRepliesAndServerRequest(t *testing.T) {
	app, server := net.Pipe()
	client := Connect(app, app, nil)
	defer client.Close()
	defer server.Close()
	client.Register("workspace/configuration", func(context.Context, json.RawMessage) (any, error) {
		return []any{map[string]any{"enabled": true}}, nil
	})
	result := make(chan error, 1)
	go func() {
		r := bufio.NewReader(server)
		first, err := readFrame(r)
		if err != nil {
			result <- err
			return
		}
		var request packet
		json.Unmarshal(first, &request)
		query := []byte(`{"jsonrpc":"2.0","id":"config-1","method":"workspace/configuration","params":{}}`)
		frame := []byte("Content-Length: " + stringInt(len(query)) + "\r\n\r\n")
		for _, part := range [][]byte{frame[:8], append(frame[8:], query[:12]...), query[12:]} {
			if _, err = server.Write(part); err != nil {
				result <- err
				return
			}
		}
		reply, err := readFrame(r)
		if err != nil {
			result <- err
			return
		}
		if !strings.Contains(string(reply), `"enabled":true`) {
			result <- errors.New("server request not answered")
			return
		}
		response, _ := json.Marshal(packet{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`"done"`)})
		result <- writeFrame(server, response)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var answer string
	if err := client.Call(ctx, "initialize", map[string]any{"processId": 1}, &answer); err != nil || answer != "done" {
		t.Fatalf("%q: %v", answer, err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentRequestsAreMatchedOutOfOrder(t *testing.T) {
	app, server := net.Pipe()
	defer server.Close()
	client := Connect(app, app, nil)
	defer client.Close()
	go func() {
		reader := bufio.NewReader(server)
		requests := make([]packet, 16)
		for i := range requests {
			body, err := readFrame(reader)
			if err != nil {
				return
			}
			_ = json.Unmarshal(body, &requests[i])
		}
		for i := len(requests) - 1; i >= 0; i-- {
			body, _ := json.Marshal(packet{JSONRPC: "2.0", ID: requests[i].ID, Result: requests[i].Params})
			if writeFrame(server, body) != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var group sync.WaitGroup
	for i := range 16 {
		group.Go(func() {
			var result int
			if err := client.Call(ctx, "echo", i, &result); err != nil || result != i {
				t.Errorf("request %d returned %d: %v", i, result, err)
			}
		})
	}
	group.Wait()
}
func stringInt(n int) string { data, _ := json.Marshal(n); return string(data) }
func TestCancellationDoesNotKillResponsiveServer(t *testing.T) {
	app, server := net.Pipe()
	client := Connect(app, app, nil)
	defer client.Close()
	defer server.Close()
	canceled := make(chan bool, 1)
	go func() {
		r := bufio.NewReader(server)
		readFrame(r) // first request deliberately has no reply
		data, _ := readFrame(r)
		canceled <- strings.Contains(string(data), "$/cancelRequest")
		data, _ = readFrame(r)
		var request packet
		json.Unmarshal(data, &request)
		answer, _ := json.Marshal(packet{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage("7")})
		writeFrame(server, answer)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := client.Call(ctx, "slow", nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel: %v", err)
	}
	if !<-canceled {
		t.Fatal("missing cancellation notification")
	}
	next, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	var answer int
	if err := client.Call(next, "next", nil, &answer); err != nil || answer != 7 {
		t.Fatalf("server died: %v", err)
	}
}
func TestMalformedFrameAndBlockedWriteAreBounded(t *testing.T) {
	for _, header := range []string{"Content-Length: 999999999\r\n\r\n", "Content-Length: 1\r\nContent-Length: 1\r\n\r\n", "Bad\r\n\r\n"} {
		if _, err := readFrame(bufio.NewReader(strings.NewReader(header))); err == nil {
			t.Fatalf("accepted %q", header)
		}
	}
	app, server := net.Pipe()
	defer server.Close()
	client := Connect(app, app, nil)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := client.Notify(ctx, "blocked", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked write: %v", err)
	}
}
