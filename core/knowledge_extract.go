package core

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

const MaxKnowledgeDocumentBytes int64 = 50 << 20

var knowledgeWhitespace = regexp.MustCompile(`[\t\f\r ]+`)
var knowledgeBlankLines = regexp.MustCompile(`\n{3,}`)

// ExtractKnowledgeText converts one of the supported knowledge document
// formats to normalized UTF-8 text. PDF extraction supports ordinary literal
// and hex text operators in compressed or uncompressed content streams; PDFs
// that contain only scanned images still require OCR before upload.
func ExtractKnowledgeText(filename, mimeType string, reader io.Reader) (string, error) {
	limited := io.LimitReader(reader, MaxKnowledgeDocumentBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if int64(len(raw)) > MaxKnowledgeDocumentBytes {
		return "", fmt.Errorf("knowledge document exceeds the %d byte extraction limit", MaxKnowledgeDocumentBytes)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))

	var text string
	switch {
	case ext == ".txt" || ext == ".md" || ext == ".markdown" || mimeType == "text/plain" || mimeType == "text/markdown":
		text = string(raw)
	case ext == ".html" || ext == ".htm" || mimeType == "text/html" || mimeType == "application/xhtml+xml":
		text, err = extractKnowledgeHTML(raw)
	case ext == ".docx" || mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		text, err = extractKnowledgeDOCX(raw)
	case ext == ".pdf" || mimeType == "application/pdf":
		text, err = extractKnowledgePDF(raw)
	default:
		err = fmt.Errorf("unsupported knowledge document type %q", firstNonemptyKnowledgeValue(mimeType, ext))
	}
	if err != nil {
		return "", err
	}

	text = normalizeKnowledgeText(text)
	if text == "" {
		return "", errors.New("the document contains no extractable text")
	}
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "�")
	}

	return text, nil
}

func firstNonemptyKnowledgeValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "unknown"
}

func normalizeKnowledgeText(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(knowledgeWhitespace.ReplaceAllString(line, " "))
	}
	value = strings.Join(lines, "\n")
	value = knowledgeBlankLines.ReplaceAllString(value, "\n\n")
	return strings.TrimSpace(value)
}

func extractKnowledgeHTML(raw []byte) (string, error) {
	doc, err := xhtml.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			switch node.Data {
			case "script", "style", "noscript", "svg":
				return
			case "br", "p", "div", "li", "tr", "section", "article", "header", "footer", "h1", "h2", "h3", "h4", "h5", "h6":
				builder.WriteByte('\n')
			}
		}
		if node.Type == xhtml.TextNode {
			builder.WriteString(node.Data)
			builder.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if node.Type == xhtml.ElementNode {
			switch node.Data {
			case "p", "div", "li", "tr", "section", "article", "h1", "h2", "h3", "h4", "h5", "h6":
				builder.WriteByte('\n')
			}
		}
	}
	walk(doc)

	return html.UnescapeString(builder.String()), nil
}

func extractKnowledgeDOCX(raw []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("invalid DOCX archive: %w", err)
	}

	var document *zip.File
	for _, file := range archive.File {
		if file.Name == "word/document.xml" {
			document = file
			break
		}
	}
	if document == nil {
		return "", errors.New("invalid DOCX: word/document.xml is missing")
	}

	reader, err := document.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	decoder := xml.NewDecoder(io.LimitReader(reader, MaxKnowledgeDocumentBytes+1))
	var builder strings.Builder
	textDepth := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("invalid DOCX XML: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "t", "instrText":
				textDepth++
			case "tab":
				builder.WriteByte('\t')
			case "br", "cr":
				builder.WriteByte('\n')
			}
		case xml.CharData:
			if textDepth > 0 {
				builder.Write([]byte(value))
			}
		case xml.EndElement:
			if (value.Name.Local == "t" || value.Name.Local == "instrText") && textDepth > 0 {
				textDepth--
			} else if value.Name.Local == "p" || value.Name.Local == "tr" {
				builder.WriteByte('\n')
			}
		}
	}

	return builder.String(), nil
}

func extractKnowledgePDF(raw []byte) (string, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("%PDF-")) {
		return "", errors.New("invalid PDF header")
	}

	var builder strings.Builder
	offset := 0
	for {
		relativeStart := bytes.Index(raw[offset:], []byte("stream"))
		if relativeStart < 0 {
			break
		}
		streamMarker := offset + relativeStart
		dataStart := streamMarker + len("stream")
		if dataStart < len(raw) && raw[dataStart] == '\r' {
			dataStart++
		}
		if dataStart < len(raw) && raw[dataStart] == '\n' {
			dataStart++
		}
		relativeEnd := bytes.Index(raw[dataStart:], []byte("endstream"))
		if relativeEnd < 0 {
			break
		}
		dataEnd := dataStart + relativeEnd
		stream := bytes.TrimRight(raw[dataStart:dataEnd], "\r\n")

		dictStart := streamMarker - 1024
		if dictStart < 0 {
			dictStart = 0
		}
		dictionary := raw[dictStart:streamMarker]
		if bytes.Contains(dictionary, []byte("/FlateDecode")) {
			inflated, inflateErr := inflatePDFStream(stream)
			if inflateErr == nil {
				stream = inflated
			}
		}

		text := parsePDFTextOperators(stream)
		if text != "" {
			builder.WriteString(text)
			builder.WriteByte('\n')
		}
		offset = dataEnd + len("endstream")
	}

	if strings.TrimSpace(builder.String()) == "" {
		return "", errors.New("the PDF has no supported text layer; scanned or custom-encoded PDFs require OCR")
	}

	return builder.String(), nil
}

func inflatePDFStream(stream []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(stream))
	if err == nil {
		defer reader.Close()
		return io.ReadAll(reader)
	}

	rawReader := flate.NewReader(bytes.NewReader(stream))
	defer rawReader.Close()
	return io.ReadAll(rawReader)
}

type pdfTextToken struct {
	kind string
	text string
}

func parsePDFTextOperators(content []byte) string {
	tokens := tokenizePDFContent(content)
	var builder strings.Builder
	for i, token := range tokens {
		if token.kind != "word" {
			continue
		}
		switch token.text {
		case "Tj", "TJ", "'", "\"":
			for j := i - 1; j >= 0 && i-j <= 4; j-- {
				if tokens[j].kind == "text" {
					builder.WriteString(tokens[j].text)
					builder.WriteByte(' ')
					break
				}
			}
		case "T*", "Td", "TD", "ET":
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func tokenizePDFContent(content []byte) []pdfTextToken {
	tokens := make([]pdfTextToken, 0)
	for i := 0; i < len(content); {
		if isPDFWhitespace(content[i]) {
			i++
			continue
		}
		switch content[i] {
		case '%':
			for i < len(content) && content[i] != '\n' && content[i] != '\r' {
				i++
			}
		case '(':
			decoded, next := parsePDFLiteralString(content, i)
			tokens = append(tokens, pdfTextToken{kind: "text", text: decodePDFString(decoded)})
			i = next
		case '<':
			if i+1 < len(content) && content[i+1] == '<' {
				i += 2
				continue
			}
			end := bytes.IndexByte(content[i+1:], '>')
			if end < 0 {
				return tokens
			}
			rawHex := bytes.Map(func(r rune) rune {
				if isPDFWhitespace(byte(r)) {
					return -1
				}
				return r
			}, content[i+1:i+1+end])
			if len(rawHex)%2 != 0 {
				rawHex = append(rawHex, '0')
			}
			decoded := make([]byte, hex.DecodedLen(len(rawHex)))
			if _, err := hex.Decode(decoded, rawHex); err == nil {
				tokens = append(tokens, pdfTextToken{kind: "text", text: decodePDFString(decoded)})
			}
			i += end + 2
		default:
			start := i
			for i < len(content) && !isPDFWhitespace(content[i]) && !strings.ContainsRune("()<>[]{}%", rune(content[i])) {
				i++
			}
			if start == i {
				i++
				continue
			}
			tokens = append(tokens, pdfTextToken{kind: "word", text: string(content[start:i])})
		}
	}
	return tokens
}

func parsePDFLiteralString(content []byte, start int) ([]byte, int) {
	result := make([]byte, 0)
	depth := 1
	for i := start + 1; i < len(content); i++ {
		value := content[i]
		if value == '\\' {
			if i+1 >= len(content) {
				return result, len(content)
			}
			i++
			next := content[i]
			switch next {
			case 'n':
				result = append(result, '\n')
			case 'r':
				result = append(result, '\r')
			case 't':
				result = append(result, '\t')
			case 'b':
				result = append(result, '\b')
			case 'f':
				result = append(result, '\f')
			case '\n':
			case '\r':
				if i+1 < len(content) && content[i+1] == '\n' {
					i++
				}
			default:
				if next >= '0' && next <= '7' {
					octal := []byte{next}
					for len(octal) < 3 && i+1 < len(content) && content[i+1] >= '0' && content[i+1] <= '7' {
						i++
						octal = append(octal, content[i])
					}
					var parsed byte
					for _, digit := range octal {
						parsed = parsed*8 + digit - '0'
					}
					result = append(result, parsed)
				} else {
					result = append(result, next)
				}
			}
			continue
		}
		if value == '(' {
			depth++
			result = append(result, value)
			continue
		}
		if value == ')' {
			depth--
			if depth == 0 {
				return result, i + 1
			}
			result = append(result, value)
			continue
		}
		result = append(result, value)
	}
	return result, len(content)
}

func decodePDFString(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xfe && raw[1] == 0xff {
		values := make([]uint16, 0, (len(raw)-2)/2)
		for i := 2; i+1 < len(raw); i += 2 {
			values = append(values, uint16(raw[i])<<8|uint16(raw[i+1]))
		}
		return string(utf16.Decode(values))
	}
	if utf8.Valid(raw) {
		return string(raw)
	}
	return strings.ToValidUTF8(string(raw), "�")
}

func isPDFWhitespace(value byte) bool {
	return value == 0 || value == '\t' || value == '\n' || value == '\f' || value == '\r' || value == ' '
}
