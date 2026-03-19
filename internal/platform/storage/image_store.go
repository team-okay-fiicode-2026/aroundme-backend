package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrUnsupportedImageType = errors.New("unsupported image type")
	ErrImageTooLarge        = errors.New("image too large")
)

var allowedImageTypes = map[string]string{
	"image/heic": ".heic",
	"image/heif": ".heif",
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var allowedImageExtensions = map[string]string{
	".heic": ".heic",
	".heif": ".heif",
	".jpeg": ".jpg",
	".jpg":  ".jpg",
	".png":  ".png",
	".webp": ".webp",
}

type LocalImageStore struct {
	baseDir  string
	subDir   string
	prefix   string
	maxBytes int64
}

func NewLocalImageStore(baseDir, subDir, prefix string, maxBytes int64) (*LocalImageStore, error) {
	dir := filepath.Join(baseDir, subDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s directory: %w", subDir, err)
	}
	return &LocalImageStore{baseDir: baseDir, subDir: subDir, prefix: prefix, maxBytes: maxBytes}, nil
}

func (s *LocalImageStore) Save(file *multipart.FileHeader) (string, error) {
	if file == nil {
		return "", nil
	}
	if file.Size > s.maxBytes {
		return "", ErrImageTooLarge
	}

	extension, err := normalizeImageExtension(file)
	if err != nil {
		return "", err
	}

	filename := s.generateFilename(extension)
	publicPath := path.Join("/uploads", s.subDir, filename)
	destinationPath := filepath.Join(s.baseDir, s.subDir, filename)

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open uploaded file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destinationPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(destinationPath)
		return "", fmt.Errorf("write file: %w", err)
	}

	return publicPath, nil
}

func (s *LocalImageStore) Delete(publicPath string) error {
	if strings.TrimSpace(publicPath) == "" {
		return nil
	}
	filename := path.Base(publicPath)
	if filename == "." || filename == "/" || filename == "" {
		return nil
	}
	targetPath := filepath.Join(s.baseDir, s.subDir, filename)
	err := os.Remove(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalImageStore) generateFilename(extension string) string {
	var randomBytes [8]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return fmt.Sprintf("%s-%d%s", s.prefix, time.Now().UnixNano(), extension)
	}
	return fmt.Sprintf(
		"%s-%s-%s%s",
		s.prefix,
		time.Now().UTC().Format("20060102t150405"),
		hex.EncodeToString(randomBytes[:]),
		extension,
	)
}

func normalizeImageExtension(file *multipart.FileHeader) (string, error) {
	contentType := strings.ToLower(strings.TrimSpace(file.Header.Get("Content-Type")))
	if extension, ok := allowedImageTypes[contentType]; ok {
		return extension, nil
	}

	extension := strings.ToLower(filepath.Ext(file.Filename))
	if normalized, ok := allowedImageExtensions[extension]; ok {
		return normalized, nil
	}

	return "", ErrUnsupportedImageType
}
