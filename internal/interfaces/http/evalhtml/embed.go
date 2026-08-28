// Package evalhtml holds the eval-page presentation assets.
//
// Two embedded file systems are exposed:
//   - TemplatesFS: HTML templates consumed by html/template (markup only)
//   - StaticFS:   CSS, JS, and other browser-served assets (no Go templates)
//
// Both are kept under separate roots so that template parsers and static
// servers can be wired independently and so that the browser asset tree can
// grow (extra CSS modules, new JS bundles) without touching template code.
package evalhtml

import "embed"

// TemplatesFS contains the HTML templates served by /eval/*.
//
//go:embed templates/*.html
var TemplatesFS embed.FS

// StaticFS contains every browser-served asset under static/ (CSS, JS).
//
//go:embed static
var StaticFS embed.FS
