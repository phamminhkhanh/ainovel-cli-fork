package web

import (
	"encoding/json"
	"sync"

	"github.com/voocel/ainovel-cli/internal/host"
)

// sseMessage 是下行 SSE 帧的统一信封。前端按 Type 分派。
// 同一信封同时服务 /api/events（实时）与 /api/replay（回放），两端处理逻辑一致。
type sseMessage struct {
	Type string          `json:"type"`           // hello|stream|clear|event|snapshot|done|ask|ask-cancel
	Text string          `json:"text,omitempty"` // stream 的增量文本
	Data json.RawMessage `json:"data,omitempty"` // event / snapshot 的结构体
	Seq  int64           `json:"seq,omitempty"`  // 回放游标（仅 replay 用）
}

// hub 把 Host 的三条单消费通道扇出给所有 SSE 订阅者。
// 关键约束：Host 的 Events/Stream/Done 必须由唯一 goroutine 消费——即 run()。
type hub struct {
	mu   sync.Mutex
	subs map[chan sseMessage]struct{}
}

func newHub() *hub {
	return &hub{subs: make(map[chan sseMessage]struct{})}
}

func (h *hub) subscribe() chan sseMessage {
	ch := make(chan sseMessage, 512)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan sseMessage) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// broadcast 非阻塞扇出：订阅者缓冲满则丢弃该帧（前端可用 /api/snapshot 或刷新经 /api/replay 重建）。
// 不能阻塞——否则会卡住唯一消费 goroutine，进而拖死引擎事件排空。
func (h *hub) broadcast(msg sseMessage) {
	h.mu.Lock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
		}
	}
	h.mu.Unlock()
}

// run 是 Host 三通道的唯一消费者。任一通道关闭（Host.Close）即退出。
// Done 是 per-run 缓冲信号（非关闭）：每次运行结束推终态快照后 hub 继续服务。
func (h *hub) run(eng *host.Host) {
	for {
		select {
		case ev, ok := <-eng.Events():
			if !ok {
				return
			}
			h.broadcast(sseMessage{Type: "event", Data: mustJSON(ev)})
			// snapshot ở done + hello (reconnect), KHÔNG mỗi event để tránh race + giảm load.
		case delta, ok := <-eng.Stream():
			if !ok {
				return
			}
			if delta == host.StreamClearSentinel {
				h.broadcast(sseMessage{Type: "clear"})
				continue
			}
			if delta == "" {
				continue
			}
			h.broadcast(sseMessage{Type: "stream", Text: delta})
		case _, ok := <-eng.Done():
			if !ok {
				return
			}
			h.broadcast(sseMessage{Type: "snapshot", Data: mustJSON(eng.Snapshot())})
			h.broadcast(sseMessage{Type: "done"})
		}
	}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}
