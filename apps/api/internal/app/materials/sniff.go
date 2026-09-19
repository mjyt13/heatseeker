package materials

import (
	"bytes"
	"mime"
	"path"
	"strings"
	"unicode/utf8"

	"heatseeker/api/internal/domain"
)

// sniffLen is how many leading bytes are inspected.
const sniffLen = 512

var (
	magicPDF  = []byte("%PDF-")
	magicPNG  = []byte("\x89PNG\r\n\x1a\n")
	magicJPEG = []byte{0xFF, 0xD8, 0xFF}
	magicZip  = []byte("PK\x03\x04")
	magicOLE  = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	magicRTF  = []byte(`{\rtf`)
	magicID3  = []byte("ID3")
	magicOgg  = []byte("OggS")
	magicFLAC = []byte("fLaC")
	magicEBML = []byte{0x1A, 0x45, 0xDF, 0xA3} // Matroska / WebM
)

// isoBoxes are the first box types of MP4/MOV/M4A files (ISO BMFF and old
// QuickTime files that start without "ftyp").
var isoBoxes = map[string]bool{"ftyp": true, "moov": true, "mdat": true, "wide": true, "free": true, "skip": true}

// executable signatures are refused whatever the extension says.
var executables = [][]byte{
	[]byte("MZ"),             // PE / DOS
	[]byte("\x7fELF"),        // ELF
	[]byte("#!"),             // scripts
	{0xCF, 0xFA, 0xED, 0xFE}, // Mach-O 64
	{0xCE, 0xFA, 0xED, 0xFE}, // Mach-O 32
	{0xCA, 0xFE, 0xBA, 0xBE}, // Mach-O fat / Java class
	[]byte("dex\n"),          // Android
}

// Extension returns the lower-case extension without the dot.
func Extension(name string) string {
	return strings.ToLower(strings.TrimPrefix(path.Ext(strings.TrimSpace(name)), "."))
}

// MimeFor picks the MIME type from the extension, falling back to the
// client's claim and finally to application/octet-stream.
func MimeFor(ext, claimed string) string {
	if t, ok := knownMime[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension("." + ext); t != "" {
		if base, _, err := mime.ParseMediaType(t); err == nil {
			return base
		}
	}
	if claimed = strings.TrimSpace(claimed); claimed != "" {
		if base, _, err := mime.ParseMediaType(claimed); err == nil && !strings.Contains(base, "html") && !strings.Contains(base, "javascript") {
			return base
		}
	}
	return "application/octet-stream"
}

var knownMime = map[string]string{
	"pdf":  "application/pdf",
	"doc":  "application/msword",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"ppt":  "application/vnd.ms-powerpoint",
	"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"xls":  "application/vnd.ms-excel",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"rtf":  "application/rtf",
	"odt":  "application/vnd.oasis.opendocument.text",
	"odp":  "application/vnd.oasis.opendocument.presentation",
	"ods":  "application/vnd.oasis.opendocument.spreadsheet",
	"txt":  "text/plain",
	"md":   "text/markdown",
	"csv":  "text/csv",
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"webp": "image/webp",
	"heic": "image/heic",
	"zip":  "application/zip",
	"mp3":  "audio/mpeg",
	"m4a":  "audio/mp4",
	"ogg":  "audio/ogg",
	"flac": "audio/flac",
	"wav":  "audio/wav",
	"mp4":  "video/mp4",
	"mov":  "video/quicktime",
	"webm": "video/webm",
	"mkv":  "video/x-matroska",
}

// CheckContent verifies that the leading bytes of a file match its extension
// and are not an executable (docs/PLAN.md §5.4).
func CheckContent(ext string, head []byte) error {
	if len(head) == 0 {
		return domain.Invalid("file", "file is empty")
	}
	for _, sig := range executables {
		if bytes.HasPrefix(head, sig) {
			return domain.Invalid("file", "executable files are not allowed")
		}
	}
	ok := true
	switch ext {
	case "pdf":
		// Some generators put junk before the header; the spec allows 1024 bytes.
		ok = bytes.Contains(head, magicPDF)
	case "png":
		ok = bytes.HasPrefix(head, magicPNG)
	case "jpg", "jpeg":
		ok = bytes.HasPrefix(head, magicJPEG)
	case "docx", "xlsx", "pptx", "odt", "ods", "odp", "zip":
		ok = bytes.HasPrefix(head, magicZip)
	case "doc", "xls", "ppt":
		ok = bytes.HasPrefix(head, magicOLE)
	case "rtf":
		ok = bytes.HasPrefix(head, magicRTF)
	case "webp":
		ok = len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP"
	case "heic":
		ok = len(head) >= 12 && string(head[4:8]) == "ftyp"
	case "txt", "md", "csv":
		ok = looksLikeText(head)
	case "mp3":
		// An ID3 tag or a bare MPEG audio frame (11 sync bits).
		ok = bytes.HasPrefix(head, magicID3) || (len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0)
	case "m4a", "mp4", "mov":
		ok = len(head) >= 8 && isoBoxes[string(head[4:8])]
	case "ogg":
		ok = bytes.HasPrefix(head, magicOgg)
	case "flac":
		ok = bytes.HasPrefix(head, magicFLAC)
	case "wav":
		ok = len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE"
	case "webm", "mkv":
		ok = bytes.HasPrefix(head, magicEBML)
	}
	if !ok {
		return domain.Invalid("file", "file content does not match its ."+ext+" extension")
	}
	return nil
}

func looksLikeText(head []byte) bool {
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	// A multi-byte rune may be cut at the end of the sniffed window.
	trimmed := head
	for i := 0; i < utf8.UTFMax && len(trimmed) > 0 && !utf8.Valid(trimmed); i++ {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return utf8.Valid(trimmed) || isLikelyCP1251(head)
}

// isLikelyCP1251 accepts legacy Windows-1251 text (common for old .txt/.csv).
func isLikelyCP1251(b []byte) bool {
	for _, c := range b {
		if c < 0x09 || (c > 0x0D && c < 0x20) {
			return false
		}
	}
	return true
}
