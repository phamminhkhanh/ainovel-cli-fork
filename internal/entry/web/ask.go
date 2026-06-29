package web

import (
	"context"
	"fmt"
	"sync"

	"github.com/voocel/ainovel-cli/internal/tools"
)

// askBridge 把阻塞式 AskUserHandler 适配为「SSE 下推问题 + HTTP 上传答案」的异步往返：
// 引擎工具线程调用 handle() 阻塞等待，浏览器渲染表单后 POST /api/ask 解阻塞。
// 等价于 TUI/headless 里同步读终端的角色，只是把同步等待换成了 channel 等待。
type askBridge struct {
	mu      sync.Mutex
	seq     int64
	pending map[string]chan *tools.AskUserResponse
	hub     *hub
}

func newAskBridge(h *hub) *askBridge {
	return &askBridge{pending: make(map[string]chan *tools.AskUserResponse), hub: h}
}

// askPayload 下推给前端的提问帧（自带 json tag → 小写键，与 tools.Question 的小写一致）。
type askPayload struct {
	ID        string           `json:"id"`
	Questions []tools.Question `json:"questions"`
}

// handle 即注入 Host 的 AskUserHandler：生成 askID、SSE 下推、阻塞等回答或 ctx 取消。
// ctx 取消（abort / 关停）返回 error，工具层据此走「自行决策继续」兜底——与无 handler 时一致。
func (b *askBridge) handle(ctx context.Context, questions []tools.Question) (*tools.AskUserResponse, error) {
	b.mu.Lock()
	b.seq++
	id := fmt.Sprintf("ask-%d", b.seq)
	ch := make(chan *tools.AskUserResponse, 1) // 缓冲 1：resolve 永不阻塞，即便 handle 已因 ctx 取消离开
	b.pending[id] = ch
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
	}()

	b.hub.broadcast(sseMessage{Type: "ask", Data: mustJSON(askPayload{ID: id, Questions: questions})})

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		// run 结束/abort 时引擎 ctx 取消：通知前端关掉这个提问模态，避免卡着无主表单。
		b.hub.broadcast(sseMessage{Type: "ask-cancel", Data: mustJSON(map[string]string{"id": id})})
		return nil, ctx.Err()
	}
}

// resolve 由 POST /api/ask 调用：把答案塞回对应 channel。返回是否命中待答问题（否则前端收 404）。
func (b *askBridge) resolve(id string, resp *tools.AskUserResponse) bool {
	b.mu.Lock()
	ch, ok := b.pending[id]
	if ok {
		delete(b.pending, id)
	}
	b.mu.Unlock()
	if !ok {
		return false
	}
	ch <- resp // 缓冲 1，不阻塞
	return true
}
