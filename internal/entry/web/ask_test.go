package web

import (
	"context"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/tools"
)

// twoOpts 是满足 ask 校验的最小问题（实际校验在工具层，这里只测桥接并发语义）。
func twoQuestions() []tools.Question {
	return []tools.Question{{
		Question: "篇幅?",
		Header:   "篇幅",
		Options:  []tools.Option{{Label: "长篇", Description: "多卷"}, {Label: "短篇", Description: "单章"}},
	}}
}

// waitAskID 轮询等待 handle() 把待答问题登记进 pending，返回其 id。
func waitAskID(t *testing.T, b *askBridge) string {
	t.Helper()
	for i := 0; i < 500; i++ {
		b.mu.Lock()
		var id string
		for k := range b.pending {
			id = k
			break
		}
		b.mu.Unlock()
		if id != "" {
			return id
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("ask 始终未登记")
	return ""
}

// TestAskBridgeResolve 验证 block→channel 往返：handle 阻塞，resolve 后拿到对应答案。
func TestAskBridgeResolve(t *testing.T) {
	b := newAskBridge(newHub())
	type result struct {
		resp *tools.AskUserResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := b.handle(context.Background(), twoQuestions())
		done <- result{resp, err}
	}()

	id := waitAskID(t, b)
	want := &tools.AskUserResponse{Answers: map[string]string{"篇幅?": "长篇"}}
	if !b.resolve(id, want) {
		t.Fatal("resolve 对在途提问应返回 true")
	}

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("handle 返回错误: %v", r.err)
		}
		if r.resp == nil || r.resp.Answers["篇幅?"] != "长篇" {
			t.Fatalf("答案未透传: %+v", r.resp)
		}
	case <-time.After(time.Second):
		t.Fatal("resolve 后 handle 未解阻塞")
	}

	// pending 应已清理。
	b.mu.Lock()
	n := len(b.pending)
	b.mu.Unlock()
	if n != 0 {
		t.Fatalf("pending 未清理: %d", n)
	}
}

// TestAskBridgeCtxCancel 验证 ctx 取消（abort/关停）能解阻塞并清理，返回 error 走工具兜底。
func TestAskBridgeCtxCancel(t *testing.T) {
	b := newAskBridge(newHub())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := b.handle(ctx, twoQuestions())
		done <- err
	}()

	waitAskID(t, b)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ctx 取消后 handle 应返回非 nil error")
		}
	case <-time.After(time.Second):
		t.Fatal("ctx 取消后 handle 未返回")
	}

	b.mu.Lock()
	n := len(b.pending)
	b.mu.Unlock()
	if n != 0 {
		t.Fatalf("ctx 取消后 pending 未清理: %d", n)
	}
}

// TestAskBridgeResolveUnknown 验证对不存在的 id（已答/已取消/伪造）resolve 返回 false。
func TestAskBridgeResolveUnknown(t *testing.T) {
	b := newAskBridge(newHub())
	if b.resolve("ask-nope", &tools.AskUserResponse{}) {
		t.Fatal("未知 id 的 resolve 应返回 false")
	}
}
