package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestChatConcurrency verifies that chat_concurrency bounds how many chat
// requests are in flight at once, and that 0 leaves them uncapped. The handler
// holds each request open briefly so concurrent calls overlap.
func TestChatConcurrency(t *testing.T) {
	empty := ""
	run := func(conc int) int {
		var mu sync.Mutex
		var inFlight, maxInFlight int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			inFlight++
			if inFlight > maxInFlight {
				maxInFlight = inFlight
			}
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			inFlight--
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
		}))
		defer srv.Close()

		c := newLLMClient(LLMConfig{
			ChatConcurrency: conc,
			Models: []LLMModel{
				{Name: "chat", Kind: "chat", Endpoint: srv.URL, Model: "m", APIPath: &empty},
			},
		}, time.Second, nil)

		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := c.Chat(context.Background(), "", "hi"); err != nil {
					t.Errorf("chat: %v", err)
				}
			}()
		}
		wg.Wait()

		mu.Lock()
		defer mu.Unlock()
		return maxInFlight
	}

	if got := run(1); got > 1 {
		t.Errorf("cap 1: max in flight = %d, want <= 1", got)
	}
	if got := run(2); got > 2 {
		t.Errorf("cap 2: max in flight = %d, want <= 2", got)
	}
	if got := run(0); got < 2 {
		t.Errorf("uncapped: max in flight = %d, want >= 2 (overlap)", got)
	}
}
