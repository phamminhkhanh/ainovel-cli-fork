package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type radarHTTPSource struct {
	id     string
	market string
	fetch  func(context.Context, *http.Client) ([]radarEntry, error)
	client *http.Client
}

const radarBrowserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func (s radarHTTPSource) ID() string     { return s.id }
func (s radarHTTPSource) Market() string { return s.market }
func (s radarHTTPSource) Fetch(ctx context.Context) ([]radarEntry, error) {
	return s.fetch(ctx, s.client)
}

func defaultRadarSources() []radarSource {
	client := &http.Client{Timeout: 15 * time.Second}
	return []radarSource{
		radarHTTPSource{id: "fanqie", market: "cn", fetch: fetchFanqieRadar, client: client},
		radarHTTPSource{id: "qidian", market: "cn", fetch: fetchQidianRadar, client: client},
	}
}

func fetchFanqieRadar(ctx context.Context, client *http.Client) ([]radarEntry, error) {
	lists := []struct {
		typeID int
		label  string
	}{{10, "hot"}, {13, "dark_horse"}}
	var entries []radarEntry
	var firstErr error
	for _, list := range lists {
		url := fmt.Sprintf("https://api-lf.fanqiesdk.com/api/novel/channel/homepage/rank/rank_list/v2/?aid=13&limit=15&offset=0&side_type=%d", list.typeID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", radarBrowserUserAgent)
		res, err := client.Do(req)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		_ = res.Body.Close()
		if readErr != nil || res.StatusCode != http.StatusOK {
			if firstErr == nil {
				firstErr = fmt.Errorf("fanqie %s: HTTP %d", list.label, res.StatusCode)
			}
			continue
		}
		var payload struct {
			Data struct {
				Result []map[string]any `json:"result"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for i, item := range payload.Data.Result {
			title := strings.TrimSpace(fmt.Sprint(item["book_name"]))
			if title == "" || title == "<nil>" {
				continue
			}
			entries = append(entries, radarEntry{
				Rank: i + 1, Title: title, Author: cleanRadarValue(item["author"]),
				Category: cleanRadarValue(item["category"]), List: list.label,
			})
		}
	}
	if len(entries) > 0 && firstErr != nil {
		return entries, firstErr // retain partial evidence but report source degraded
	}
	if len(entries) == 0 && firstErr != nil {
		return nil, firstErr
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("fanqie: no ranking entries parsed")
	}
	return entries, nil
}

var qidianBookPattern = regexp.MustCompile(`(?i)<a[^>]+href=["'](?:https?:)?//book\.qidian\.com/info/\d+[^"']*["'][^>]*>([^<]+)</a>`)

func fetchQidianRadar(ctx context.Context, client *http.Client) ([]radarEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.qidian.com/rank/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", radarBrowserUserAgent)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qidian: HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 3<<20))
	if err != nil {
		return nil, err
	}
	matches := qidianBookPattern.FindAllSubmatch(body, -1)
	seen := make(map[string]struct{})
	entries := make([]radarEntry, 0, 20)
	for _, match := range matches {
		title := strings.TrimSpace(html.UnescapeString(string(match[1])))
		if title == "" || len([]rune(title)) > 50 {
			continue
		}
		if _, ok := seen[title]; ok {
			continue
		}
		seen[title] = struct{}{}
		entries = append(entries, radarEntry{Rank: len(entries) + 1, Title: title, List: "rank"})
		if len(entries) == 20 {
			break
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("qidian: no ranking entries parsed")
	}
	return entries, nil
}

func cleanRadarValue(value any) string {
	s := strings.TrimSpace(fmt.Sprint(value))
	if s == "<nil>" {
		return ""
	}
	return s
}
