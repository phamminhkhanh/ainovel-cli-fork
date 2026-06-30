package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/voocel/ainovel-cli/internal/store"
)

// MaxChapterBytes giới hạn kích thước tối đa của một chương/draft được đọc.
// Giới hạn này bảo vệ server khỏi đọc file khổng lồ vào bộ nhớ / trả về response quá lớn.
const MaxChapterBytes = 1 << 20 // 1 MB

// contentEngine cung cấp store để content handlers đọc dữ liệu trên đĩa.
// Dùng interface nhỏ này để test không phải khởi tạo toàn bộ Host.
type contentEngine interface {
	Store() *store.Store
}

// chapterResponse là dạng thức chung cho chương chính thức lẫn draft.
type chapterResponse struct {
	Chapter int    `json:"chapter"`
	Kind    string `json:"kind"` // "final" | "draft"
	Text    string `json:"text"`
}

// outlineResponse gom premise / outline / layered outline / compass.
type outlineResponse struct {
	Premise string `json:"premise"`
	Outline any    `json:"outline"`
	Layered any    `json:"layered"`
	Compass any    `json:"compass"`
}

// worldResponse gom world rules / timeline / compass.
type worldResponse struct {
	Rules    any `json:"rules"`
	Timeline any `json:"timeline"`
	Compass  any `json:"compass"`
}

// charactersResponse gom nhân vật chính + supporting cast gần đây.
type charactersResponse struct {
	Characters any `json:"characters"`
	Supporting any `json:"supporting"`
}

func (s *server) handleChapter(w http.ResponseWriter, r *http.Request) {
	serveChapter(s, w, r, false)
}

func (s *server) handleChapterDraft(w http.ResponseWriter, r *http.Request) {
	serveChapter(s, w, r, true)
}

func (s *server) handleOutline(w http.ResponseWriter, r *http.Request) {
	serveOutline(s, w, r)
}

func (s *server) handleWorld(w http.ResponseWriter, r *http.Request) {
	serveWorld(s, w, r)
}

func (s *server) handleCharacters(w http.ResponseWriter, r *http.Request) {
	serveCharacters(s, w, r)
}

func serveChapter(eng contentEngine, w http.ResponseWriter, r *http.Request, draft bool) {
	n, ok := chapterParam(w, r)
	if !ok {
		return
	}

	st := eng.Store()
	if err := checkChapterSize(st.Dir(), n, draft); err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, err)
		return
	}

	var text string
	var err error
	if draft {
		text, err = st.Drafts.LoadDraft(n)
	} else {
		text, err = st.Drafts.LoadChapterText(n)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(text) == "" {
		writeErr(w, http.StatusNotFound, fmt.Errorf("chapter not found"))
		return
	}

	kind := "final"
	if draft {
		kind = "draft"
	}
	writeJSON(w, http.StatusOK, chapterResponse{Chapter: n, Kind: kind, Text: text})
}

// checkChapterSize ngăn đọc file quá lớn trước khi load vào bộ nhớ.
func checkChapterSize(dir string, n int, draft bool) error {
	rel := fmt.Sprintf("chapters/%02d.md", n)
	if draft {
		rel = fmt.Sprintf("drafts/%02d.draft.md", n)
	}
	info, err := os.Stat(filepath.Join(dir, rel))
	if err != nil {
		return nil // file không tồn tại: để loader trả 404
	}
	if info.Size() > MaxChapterBytes {
		return fmt.Errorf("chapter exceeds %d byte limit", MaxChapterBytes)
	}
	return nil
}

func serveOutline(eng contentEngine, w http.ResponseWriter, r *http.Request) {
	st := eng.Store()
	premise, err := st.Outline.LoadPremise()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	outline, err := st.Outline.LoadOutline()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	layered, err := st.Outline.LoadLayeredOutline()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	compass, err := st.Outline.LoadCompass()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, outlineResponse{
		Premise: premise,
		Outline: outline,
		Layered: layered,
		Compass: compass,
	})
}

func serveWorld(eng contentEngine, w http.ResponseWriter, r *http.Request) {
	st := eng.Store()
	rules, err := st.World.LoadWorldRules()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	timeline, err := st.World.LoadTimeline()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	compass, err := st.Outline.LoadCompass()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, worldResponse{
		Rules:    rules,
		Timeline: timeline,
		Compass:  compass,
	})
}

func serveCharacters(eng contentEngine, w http.ResponseWriter, r *http.Request) {
	st := eng.Store()
	chars, err := st.Characters.Load()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	cast, err := st.Cast.RecentActive(50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, charactersResponse{
		Characters: chars,
		Supporting: cast,
	})
}

// chapterParam parse và validate path param "n".
func chapterParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	v := r.PathValue("n")
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid chapter number"))
		return 0, false
	}
	return n, true
}
