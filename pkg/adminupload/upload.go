package adminupload

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	imageExts = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true,
	}
	excelExts = map[string]bool{".xlsx": true, ".xls": true}
)

type SavedFile struct {
	RelPath  string
	URL      string
	Filename string
	Size     int64
}

func AllowedImage(name string) bool {
	return imageExts[strings.ToLower(filepath.Ext(name))]
}

func AllowedExcel(name string) bool {
	return excelExts[strings.ToLower(filepath.Ext(name))]
}

func SaveUploaded(dir, publicBase string, header *multipart.FileHeader, allow func(string) bool) (*SavedFile, error) {
	if header == nil {
		return nil, fmt.Errorf("file required")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allow(header.Filename) {
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	subdir := time.Now().UTC().Format("2006/01")
	targetDir := filepath.Join(dir, subdir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, err
	}

	safeName := uuid.NewString() + ext
	absPath := filepath.Join(targetDir, safeName)

	src, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	dst, err := os.Create(absPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		return nil, err
	}

	rel := filepath.ToSlash(filepath.Join(subdir, safeName))
	url := strings.TrimRight(publicBase, "/") + "/" + rel
	return &SavedFile{
		RelPath:  rel,
		URL:      url,
		Filename: header.Filename,
		Size:     n,
	}, nil
}
