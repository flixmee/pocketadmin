package apis

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/spf13/cast"
)

const mcpMediaDownloadTimeout = 30 * time.Second

var errMCPMediaFolderNotFound = errors.New("media folder doesn't exist")

// mcpMediaHTTPClient is used to download remote files for upload_media.
var mcpMediaHTTPClient = &http.Client{Timeout: mcpMediaDownloadTimeout}

// mcpMediaItem serializes a media record into the shape returned by the MCP media tools.
func mcpMediaItem(collection *core.Collection, record *core.Record, mediaPath string) map[string]any {
	item := map[string]any{
		"id":      record.Id,
		"name":    record.GetString("name"),
		"kind":    record.GetString("kind"),
		"path":    mediaPath,
		"created": record.GetDateTime("created"),
	}

	if record.GetString("kind") == core.MediaKindFile {
		item["url"] = "/api/files/" + url.PathEscape(collection.Id) + "/" + url.PathEscape(record.Id) + "/" + url.PathEscape(record.GetString("file"))
		item["mime"] = record.GetString("mime")
		item["size"] = record.GetInt("size")
	}

	return item
}

// resolveMCPMediaFolder returns the id of the folder at the specified path
// ("" for the root). When create is true, missing folders are created.
func (s *mcpServer) resolveMCPMediaFolder(collection *core.Collection, folderPath string, create bool) (string, string, error) {
	folderPath = core.NormalizeMediaPath(folderPath)
	if folderPath == "" || folderPath == "/" {
		return "", "", nil
	}

	parentId := ""
	for _, segment := range strings.Split(strings.TrimPrefix(folderPath, "/"), "/") {
		existing, err := s.app.FindAllRecords(collection, dbx.HashExp{"parent": parentId, "name": segment})
		if err != nil {
			return "", "", err
		}

		if len(existing) > 0 {
			if existing[0].GetString("kind") != core.MediaKindFolder {
				return "", "", fmt.Errorf("%q is a file, not a folder", segment)
			}
			parentId = existing[0].Id
			continue
		}

		if !create {
			return "", "", fmt.Errorf("%w: %q", errMCPMediaFolderNotFound, folderPath)
		}

		folder := core.NewRecord(collection)
		folder.Set("kind", core.MediaKindFolder)
		folder.Set("name", segment)
		folder.Set("parent", parentId)
		if err := s.app.Save(folder); err != nil {
			return "", "", fmt.Errorf("failed to create media folder %q: %w", segment, err)
		}
		parentId = folder.Id
	}

	return parentId, folderPath, nil
}

// uniqueMCPMediaName returns name, or a "name-N.ext" variant when an item
// with the same name already exists in the parent folder.
func (s *mcpServer) uniqueMCPMediaName(collection *core.Collection, parentId string, name string) (string, error) {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)

	candidate := name
	for i := 1; i <= 100; i++ {
		existing, err := s.app.FindAllRecords(collection, dbx.HashExp{"parent": parentId, "name": candidate})
		if err != nil {
			return "", err
		}
		if len(existing) == 0 {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d%s", base, i, ext)
	}

	return "", fmt.Errorf("a media item named %q already exists", name)
}

func mcpMediaMaxSize(collection *core.Collection) int64 {
	if field, ok := collection.Fields.GetByName("file").(*core.FileField); ok && field.MaxSize > 0 {
		return field.MaxSize
	}

	return core.DefaultFileFieldMaxSize
}

// decodeMCPContent decodes the upload_media "content" argument.
//
// With encoding "text" the content is stored as is. Otherwise it is
// treated as base64 (standard or URL-safe, with or without padding),
// optionally prefixed with a data URI header. Raw markup (e.g. an SVG)
// and non-base64 data URIs are detected and stored as text.
func decodeMCPContent(content string, encoding string) ([]byte, error) {
	switch strings.ToLower(encoding) {
	case "", "base64":
	case "text", "utf8", "utf-8":
		return []byte(content), nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q (use \"base64\" or \"text\")", encoding)
	}

	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "data:") {
		header, payload, ok := strings.Cut(content[len("data:"):], ",")
		if !ok {
			return nil, errors.New("argument \"content\" is an invalid data URI (missing \",\")")
		}

		if !strings.HasSuffix(strings.ToLower(header), ";base64") {
			// e.g. data:image/svg+xml;utf8,<svg ...> or data:text/plain,hello%20world
			if unescaped, err := url.PathUnescape(payload); err == nil {
				return []byte(unescaped), nil
			}
			return []byte(payload), nil
		}

		content = payload
	}

	// raw markup can't be valid base64 ("<" is not part of the alphabet)
	if strings.HasPrefix(content, "<") {
		return []byte(content), nil
	}

	// drop whitespaces, quotes and leftovers from double JSON encoding
	content = strings.NewReplacer(`\n`, "", `\r`, "", `\t`, "", `\/`, "/", `\u003d`, "=", `\u002b`, "+").Replace(content)
	content = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ', '"', '\'':
			return -1
		}
		return r
	}, content)

	// normalize to unpadded standard base64
	content = strings.NewReplacer("-", "+", "_", "/").Replace(content)
	content = strings.TrimRight(content, "=")

	data, err := base64.RawStdEncoding.DecodeString(content)
	if err != nil {
		reason := err.Error()
		if len(content)%4 == 1 {
			reason = fmt.Sprintf("the data looks truncated (%d characters)", len(content))
		} else if corrupt, ok := err.(base64.CorruptInputError); ok && int(corrupt) < len(content) {
			reason = fmt.Sprintf("invalid character %q at position %d of %d", content[corrupt], corrupt, len(content))
		}

		return nil, fmt.Errorf("argument \"content\" is not valid base64: %s. "+
			"Send the complete file base64 encoded, set \"encoding\":\"text\" for text files such as SVG, or use \"url\" instead", reason)
	}

	return data, nil
}

// downloadMCPMediaFile downloads the remote file and returns its content
// together with a filename suggested by the response.
func downloadMCPMediaFile(ctx context.Context, rawURL string, maxSize int64) ([]byte, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, "", errors.New("argument \"url\" must be an absolute http(s) URL")
	}

	ctx, cancel := context.WithTimeout(ctx, mcpMediaDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", err
	}

	res, err := mcpMediaHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download %s: %w", rawURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, "", fmt.Errorf("failed to download %s (status %d)", rawURL, res.StatusCode)
	}

	if res.ContentLength > maxSize {
		return nil, "", fmt.Errorf("the remote file exceeds the max allowed size of %d bytes", maxSize)
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, maxSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to download %s: %w", rawURL, err)
	}
	if int64(len(data)) > maxSize {
		return nil, "", fmt.Errorf("the remote file exceeds the max allowed size of %d bytes", maxSize)
	}

	name := ""
	if _, params, err := mime.ParseMediaType(res.Header.Get("Content-Disposition")); err == nil {
		name = path.Base(params["filename"])
	}
	if name == "" || name == "." || name == "/" {
		name = path.Base(parsed.Path)
	}
	if name == "" || name == "." || name == "/" {
		name = "file"
	}

	// add an extension based on the content type if missing
	if path.Ext(name) == "" {
		if exts, _ := mime.ExtensionsByType(res.Header.Get("Content-Type")); len(exts) > 0 {
			name += exts[0]
		}
	}

	return data, name, nil
}

// -------------------------------------------------------------------

func mcpListMedia(s *mcpServer, args mcpArgs) (any, error) {
	collection, err := s.app.FindCachedCollectionByNameOrId(core.CollectionNameMedias)
	if err != nil {
		return nil, err
	}

	perPage := cast.ToInt(args["perPage"])
	if perPage <= 0 {
		perPage = 100
	}
	if perPage > mcpMaxPerPage {
		perPage = mcpMaxPerPage
	}

	page := cast.ToInt(args["page"])
	if page <= 0 {
		page = 1
	}

	query := s.app.RecordQuery(collection).
		OrderBy("kind DESC", "name ASC"). // folders first
		Limit(int64(perPage)).
		Offset(int64((page - 1) * perPage))

	items := []map[string]any{}

	if search := args.string("search"); search != "" {
		// search all files and folders by name
		records := []*core.Record{}
		if err := query.AndWhere(dbx.Like("name", search)).All(&records); err != nil {
			return nil, err
		}

		for _, record := range records {
			mediaPath, err := core.BuildMediaPath(s.app, record)
			if err != nil {
				mediaPath = ""
			}
			items = append(items, mcpMediaItem(collection, record, mediaPath))
		}

		return map[string]any{"page": page, "perPage": perPage, "items": items}, nil
	}

	parentId, folderPath, err := s.resolveMCPMediaFolder(collection, args.string("folder"), false)
	if errors.Is(err, errMCPMediaFolderNotFound) {
		// not an error for the client - upload_media creates missing folders
		return map[string]any{"folder": core.NormalizeMediaPath(args.string("folder")), "exists": false, "page": page, "perPage": perPage, "items": items}, nil
	}
	if err != nil {
		return nil, err
	}

	records := []*core.Record{}
	if err := query.AndWhere(dbx.HashExp{"parent": parentId}).All(&records); err != nil {
		return nil, err
	}

	for _, record := range records {
		items = append(items, mcpMediaItem(collection, record, strings.TrimSuffix(folderPath, "/")+"/"+record.GetString("name")))
	}

	if folderPath == "" {
		folderPath = "/"
	}

	return map[string]any{"folder": folderPath, "exists": true, "page": page, "perPage": perPage, "items": items}, nil
}

func mcpUploadMedia(s *mcpServer, args mcpArgs) (any, error) {
	collection, err := s.app.FindCachedCollectionByNameOrId(core.CollectionNameMedias)
	if err != nil {
		return nil, err
	}

	content := args.string("content")
	sourceURL := args.string("url")
	if (content == "") == (sourceURL == "") {
		return nil, errors.New("provide exactly one of \"content\" (base64) or \"url\"")
	}

	maxSize := mcpMediaMaxSize(collection)

	var data []byte
	name := args.string("name")

	if content != "" {
		if name == "" {
			return nil, errors.New("missing required argument \"name\" (e.g. \"cover.jpg\")")
		}

		data, err = decodeMCPContent(content, args.string("encoding"))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxSize {
			return nil, fmt.Errorf("the file exceeds the max allowed size of %d bytes", maxSize)
		}
	} else {
		var remoteName string
		data, remoteName, err = downloadMCPMediaFile(context.Background(), sourceURL, maxSize)
		if err != nil {
			return nil, err
		}
		if name == "" {
			name = remoteName
		}
	}

	if len(data) == 0 {
		return nil, errors.New("the file is empty")
	}

	name = strings.TrimSpace(path.Base(strings.ReplaceAll(name, `\`, "/")))
	if name == "" || name == "." || name == "/" {
		return nil, errors.New("invalid file name")
	}

	parentId, folderPath, err := s.resolveMCPMediaFolder(collection, args.string("folder"), true)
	if err != nil {
		return nil, err
	}

	name, err = s.uniqueMCPMediaName(collection, parentId, name)
	if err != nil {
		return nil, err
	}

	file, err := filesystem.NewFileFromBytes(data, name)
	if err != nil {
		return nil, err
	}

	record := core.NewRecord(collection)
	record.Set("kind", core.MediaKindFile)
	record.Set("parent", parentId)
	record.Set("name", name)
	record.Set("file", file)

	if err := s.app.Save(record); err != nil {
		return nil, err
	}

	item := mcpMediaItem(collection, record, strings.TrimSuffix(folderPath, "/")+"/"+name)
	item["hint"] = "Use the url value when setting a media field, e.g. update_record with {\"cover\":\"" + cast.ToString(item["url"]) + "\"}."

	return item, nil
}
