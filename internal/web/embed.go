package web

import "embed"

// embeddedDir is where go:embed finds the built frontend, relative to this
// package. DevDir is the same directory relative to the repository root, which
// is where DEV=1 serves it from disk: an embed path and a filesystem path
// cannot be the same string once the package is not at the root.
const embeddedDir = "public"

// DevDir is the built frontend's path from the repository root, for DEV=1.
const DevDir = "internal/web/public"

// The files are listed explicitly so a missing build output fails with the name
// of the file to rebuild (`make frontend`) instead of "contains no embeddable
// files".
//
//go:embed public/index.html public/app.js public/style.css public/favicon.png
var publicFS embed.FS
