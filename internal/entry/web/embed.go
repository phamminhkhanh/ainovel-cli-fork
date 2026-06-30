package web

import "embed"

// assetFS 把前端单页（go:embed）打进二进制，零构建步骤、可离线运行。
// 注意：这里嵌入的是本包下的 assets/ 目录，与模块根的 assets 包（创作素材）无关。
//
//go:embed assets/index.html assets/app-i18n.js assets/app.js assets/app.css assets/app-dashboard.js assets/app-workspace.js assets/app-studio.js assets/app-input.js
var assetFS embed.FS
