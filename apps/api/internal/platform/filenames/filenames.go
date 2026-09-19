// Package filenames holds helpers for user-supplied file names: safe object
// keys, Content-Disposition values and which types may render inline.
package filenames

import (
	"mime"
	"path"
	"strings"
	"unicode"
)

// ContentDisposition builds a header value that survives non-ASCII names
// (RFC 6266 / RFC 5987).
func ContentDisposition(fileName string, inline bool) string {
	kind := "attachment"
	if inline {
		kind = "inline"
	}
	if fileName == "" {
		return kind
	}
	if v := mime.FormatMediaType(kind, map[string]string{"filename": fileName}); v != "" {
		return v
	}
	return kind
}

// SafeName reduces a user-supplied file name to something safe to embed in an
// object key: no path separators, no control characters, bounded length.
func SafeName(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || unicode.IsControl(r):
			continue
		case unicode.IsSpace(r):
			b.WriteByte('_')
		case strings.ContainsRune(`"'<>:|?*#%&{}$!@+=`+"`", r):
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		out = "file"
	}
	if runes := []rune(out); len(runes) > 120 {
		ext := path.Ext(out)
		if len(ext) > 10 {
			ext = ""
		}
		out = string(runes[:120-len([]rune(ext))]) + ext
	}
	return out
}

// Inline reports whether a MIME type is safe to render in a browser: PDF,
// raster images, audio and video (played by the browser or the app's player)
// and plain text (served with nosniff). SVG and HTML can carry scripts.
func Inline(mimeType string) bool {
	switch {
	case mimeType == "application/pdf", mimeType == "text/plain":
		return true
	case strings.HasPrefix(mimeType, "image/"):
		return mimeType != "image/svg+xml"
	case strings.HasPrefix(mimeType, "audio/"), strings.HasPrefix(mimeType, "video/"):
		return true
	}
	return false
}
