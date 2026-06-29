package web

import (
	"testing"
	"time"
)

// TestHubFanOut 验证扇出：一次 broadcast 所有订阅者都收到；unsubscribe 后通道关闭，其余订阅者不受影响。
func TestHubFanOut(t *testing.T) {
	h := newHub()
	a := h.subscribe()
	b := h.subscribe()

	h.broadcast(sseMessage{Type: "event", Text: "x"})

	if got := <-a; got.Type != "event" || got.Text != "x" {
		t.Fatalf("订阅者 a 收到异常帧: %+v", got)
	}
	if got := <-b; got.Type != "event" || got.Text != "x" {
		t.Fatalf("订阅者 b 收到异常帧: %+v", got)
	}

	// 退订 a：通道应被关闭（range/recv 得到 ok=false），且不再登记。
	h.unsubscribe(a)
	if _, ok := <-a; ok {
		t.Fatal("unsubscribe 后通道应已关闭")
	}

	// b 仍在册，继续收帧。
	h.broadcast(sseMessage{Type: "done"})
	if got := <-b; got.Type != "done" {
		t.Fatalf("退订 a 后 b 仍应收帧, got %+v", got)
	}
}

// TestHubBroadcastNonBlocking 验证非阻塞扇出：订阅者从不排空时，远超缓冲(512)的广播也必须立即返回，
// 绝不能卡住——否则会拖死唯一消费 goroutine。满了即丢帧是设计预期。
func TestHubBroadcastNonBlocking(t *testing.T) {
	h := newHub()
	_ = h.subscribe() // 故意不排空

	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			h.broadcast(sseMessage{Type: "stream", Text: "delta"})
		}
		close(done)
	}()

	select {
	case <-done:
		// 通过：远超缓冲仍迅速完成。
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast 在订阅者未排空时被阻塞（应丢帧而非阻塞）")
	}
}

// TestHubUnsubscribeIdempotent 验证重复退订安全：第二次退订是 no-op，不会重复 close 通道而 panic。
func TestHubUnsubscribeIdempotent(t *testing.T) {
	h := newHub()
	a := h.subscribe()
	h.unsubscribe(a)
	h.unsubscribe(a) // 不应 panic（double-close 守卫）
}
