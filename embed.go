package taskqueue

import "embed"

// publicDirName holds the built frontend: Bun writes it, go:embed ships it.
const publicDirName = "public"

// The files are listed explicitly so a missing build output fails with the name
// of the file to rebuild (`make frontend`) instead of "contains no embeddable
// files".
//
//go:embed public/index.html public/app.js public/style.css public/favicon.png
var publicFS embed.FS
