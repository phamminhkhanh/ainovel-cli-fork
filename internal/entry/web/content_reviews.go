package web

import (
	"net/http"

	"github.com/voocel/ainovel-cli/internal/domain"
)

// Đọc-chỉ (read-only) cho hai thứ người viết cần nhìn sâu vào chất lượng & bối cảnh:
//   - Đánh giá 7 chiều của Editor (reviews/*.json)  → hiểu VÌ SAO một chương bị bắt viết lại
//   - Sổ 伏笔 (foreshadow ledger)                    → theo dõi tuyến ngầm cài/thu hồi
// Cùng khuôn với content.go: đọc thẳng qua store cache, không tạo store mới mỗi request.

// reviewsResponse gom các review đã lưu, sắp theo chương tăng dần + review global gần nhất.
type reviewsResponse struct {
	Reviews []*domain.ReviewEntry `json:"reviews"`
	Global  *domain.ReviewEntry   `json:"global"`
}

// foreshadowResponse là toàn bộ sổ 伏笔 (planted / advanced / resolved).
type foreshadowResponse struct {
	Entries []domain.ForeshadowEntry `json:"entries"`
}

func (s *server) handleReviews(w http.ResponseWriter, r *http.Request) {
	serveReviews(s, w, r)
}

func (s *server) handleForeshadow(w http.ResponseWriter, r *http.Request) {
	serveForeshadow(s, w, r)
}

// serveReviews duyệt các chương đã đi qua (theo Progress) và nạp review từng chương.
// Review chỉ tồn tại cho chương Editor đã chấm; chương chưa chấm bị bỏ qua, không phải lỗi.
//
// Hiệu năng: đọc O(số chương) file reviews/NN.json mỗi lần mở tab. Với truyện 500+ chương là
// 500+ lần đọc đĩa — chấp nhận được cho công cụ chạy local/1 người, nhưng nếu về sau thấy chậm
// thì cân nhắc phân trang lười (from/to) hoặc cache theo mtime thư mục reviews/.
func serveReviews(eng contentEngine, w http.ResponseWriter, r *http.Request) {
	st := eng.Store()

	maxCh := 0
	if prog, err := st.Progress.Load(); err == nil && prog != nil {
		maxCh = prog.CurrentChapter
		for _, c := range prog.CompletedChapters {
			if c > maxCh {
				maxCh = c
			}
		}
	}

	reviews := make([]*domain.ReviewEntry, 0, maxCh)
	for ch := 1; ch <= maxCh; ch++ {
		rv, err := st.World.LoadReview(ch)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if rv != nil {
			reviews = append(reviews, rv)
		}
	}

	global, err := st.World.LoadLastReview(maxCh)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, reviewsResponse{Reviews: reviews, Global: global})
}

func serveForeshadow(eng contentEngine, w http.ResponseWriter, r *http.Request) {
	entries, err := eng.Store().World.LoadForeshadowLedger()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, foreshadowResponse{Entries: entries})
}
