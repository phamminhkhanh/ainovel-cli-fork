package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/voocel/agentcore"
)

const radarMaxTokens = 6000

const radarSystemPrompt = `You are a commercial web-fiction market analyst.
Analyze current Chinese ranking snapshots as an early signal for the target market.
Chinese trends may transfer to Vietnam or Spanish-speaking markets after 6-12 months,
but cultural portability must be evaluated rather than assumed.

LANGUAGE CONTRACT: write every human-readable output field in the requested output language:
marketSummary, genre, concept, reasoning, localizationRisks, and evidence explanations.
Keep Chinese benchmark book titles exactly as supplied; do not translate or romanize them.

Use live evidence when available. If all sources failed, explicitly use general model knowledge
and keep confidence conservative. Do not invent ranking titles that are not in the input.

Return JSON only:
{
  "marketSummary": "concise market overview",
  "recommendations": [{
    "genre": "genre/subgenre",
    "concept": "one concrete localized story concept",
    "momentum": 0.0,
    "saturation": 0.0,
    "transferability": 0.0,
    "estimatedLagMonths": [6, 12],
    "reasoning": "why this can transfer",
    "benchmarkTitles": ["titles from supplied rankings"],
    "localizationRisks": ["specific cultural or platform risks"],
    "evidence": ["source/list/rank/title evidence"]
  }]
}

Return 3-5 recommendations sorted by transferability, then momentum. Scores are 0-1.
Prefer opportunities with rising momentum and manageable saturation, not merely today's largest genre.`

func (s *server) analyzeRadar(ctx context.Context, sourceMarket, targetMarket string, snapshots []radarSnapshot) (radarReport, error) {
	models, err := s.studioModelSet()
	if err != nil {
		return radarReport{}, fmt.Errorf("radar model init: %w", err)
	}
	payload, err := json.Marshal(struct {
		SourceMarket   string          `json:"sourceMarket"`
		TargetMarket   string          `json:"targetMarket"`
		OutputLanguage string          `json:"outputLanguage"`
		CurrentYear    int             `json:"currentYear"`
		Snapshots      []radarSnapshot `json:"snapshots"`
	}{sourceMarket, targetMarket, radarOutputLanguage(targetMarket), time.Now().Year(), snapshots})
	if err != nil {
		return radarReport{}, err
	}
	content, err := generateRadarText(ctx, models.ForRole("thinking"), radarSystemPrompt, string(payload))
	if err != nil {
		return radarReport{}, err
	}
	report, err := parseRadarReport(content)
	if err != nil {
		return radarReport{}, err
	}
	return report, nil
}

func radarOutputLanguage(targetMarket string) string {
	switch targetMarket {
	case "vi":
		return "Vietnamese (tiếng Việt tự nhiên, dễ hiểu; giữ nguyên tên sách Trung Quốc)"
	case "es":
		return "Neutral Spanish (español neutro; keep Chinese book titles unchanged)"
	default:
		return "English (keep Chinese book titles unchanged)"
	}
}

func generateRadarText(ctx context.Context, model agentcore.ChatModel, systemPrompt, userPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	stream, err := model.GenerateStream(ctx, []agentcore.Message{
		agentcore.SystemMsg(systemPrompt), agentcore.UserMsg(userPrompt),
	}, nil, agentcore.WithMaxTokens(radarMaxTokens))
	if err != nil {
		return "", fmt.Errorf("radar analyze: %w", err)
	}
	var text, thinking strings.Builder
	var streamed bool
	for event := range stream {
		switch event.Type {
		case agentcore.StreamEventThinkingDelta:
			thinking.WriteString(event.Delta)
		case agentcore.StreamEventTextDelta:
			streamed = true
			text.WriteString(event.Delta)
		case agentcore.StreamEventDone:
			if !streamed {
				text.WriteString(event.Message.TextContent())
			}
		case agentcore.StreamEventError:
			if event.Err != nil {
				return "", fmt.Errorf("radar analyze: %w", event.Err)
			}
			return "", fmt.Errorf("radar analyze failed")
		}
	}
	out := strings.TrimSpace(text.String())
	if out == "" {
		out = strings.TrimSpace(thinking.String())
	}
	if out == "" {
		return "", fmt.Errorf("radar model returned empty report")
	}
	return out, nil
}

func parseRadarReport(content string) (radarReport, error) {
	content = stripCodeFence(strings.TrimSpace(content))
	start, end := strings.IndexByte(content, '{'), strings.LastIndexByte(content, '}')
	if start < 0 || end <= start {
		return radarReport{}, fmt.Errorf("radar output has no JSON object")
	}
	var report radarReport
	if err := json.Unmarshal([]byte(content[start:end+1]), &report); err != nil {
		return radarReport{}, fmt.Errorf("parse radar report: %w", err)
	}
	for i := range report.Recommendations {
		report.Recommendations[i].Momentum = clampRadarScore(report.Recommendations[i].Momentum)
		report.Recommendations[i].Saturation = clampRadarScore(report.Recommendations[i].Saturation)
		report.Recommendations[i].Transferability = clampRadarScore(report.Recommendations[i].Transferability)
	}
	if len(report.Recommendations) > 5 {
		report.Recommendations = report.Recommendations[:5]
	}
	return report, nil
}

func clampRadarScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}
