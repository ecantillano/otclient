package launcher

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxArchiveFiles       = 100_000
	maxArchiveFileBytes   = uint64(2 << 30)
	maxArchiveTotalBytes  = uint64(8 << 30)
	diskSafetyMarginBytes = uint64(64 << 20)
)

type ExtractedFile struct {
	RelativePath string
	SourcePath   string
	Size         uint64
}

type ArchiveExtractor struct {
	seen map[string]string
}

func NewArchiveExtractor() *ArchiveExtractor {
	return &ArchiveExtractor{seen: make(map[string]string)}
}

func (extractor *ArchiveExtractor) Extract(zipFile, destinationRoot, target string) ([]ExtractedFile, uint64, error) {
	if err := os.MkdirAll(destinationRoot, 0o700); err != nil {
		return nil, 0, err
	}
	archive, err := zip.OpenReader(zipFile)
	if err != nil {
		return nil, 0, fmt.Errorf("open ZIP: %w", err)
	}
	defer archive.Close()
	if len(archive.File) > maxArchiveFiles {
		return nil, 0, fmt.Errorf("ZIP contains too many entries")
	}
	if target == "" {
		target = "."
	}

	type checkedEntry struct {
		file     *zip.File
		relative string
	}
	checked := make([]checkedEntry, 0, len(archive.File))
	var total uint64
	localSeen := make(map[string]struct{}, len(archive.File))
	for _, entry := range archive.File {
		entryName := strings.TrimSuffix(strings.ReplaceAll(entry.Name, `\`, "/"), "/")
		if entryName == "" {
			continue
		}
		relative, pathErr := SafeRelativePath(entryName)
		if pathErr != nil {
			return nil, 0, fmt.Errorf("unsafe ZIP entry %q: %w", entry.Name, pathErr)
		}
		if target != "." {
			relative, pathErr = SafeRelativePath(path.Join(target, relative))
			if pathErr != nil {
				return nil, 0, pathErr
			}
		}
		mode := entry.Mode()
		if !entry.FileInfo().IsDir() && !mode.IsRegular() {
			return nil, 0, fmt.Errorf("ZIP entry %q is not a regular file or directory", entry.Name)
		}
		key := strings.ToLower(relative)
		if _, duplicate := localSeen[key]; duplicate {
			return nil, 0, fmt.Errorf("ZIP contains duplicate or case-colliding path %q", relative)
		}
		localSeen[key] = struct{}{}
		if !entry.FileInfo().IsDir() {
			if previous, duplicate := extractor.seen[key]; duplicate {
				return nil, 0, fmt.Errorf("component path %q collides with %q", relative, previous)
			}
			if entry.UncompressedSize64 > maxArchiveFileBytes || total > maxArchiveTotalBytes-entry.UncompressedSize64 {
				return nil, 0, fmt.Errorf("ZIP uncompressed size exceeds safety limit")
			}
			total += entry.UncompressedSize64
		}
		checked = append(checked, checkedEntry{file: entry, relative: relative})
	}

	if available, supported, diskErr := availableDiskBytes(destinationRoot); diskErr != nil {
		return nil, 0, fmt.Errorf("disk space preflight: %w", diskErr)
	} else if supported && available < total+diskSafetyMarginBytes {
		return nil, 0, fmt.Errorf("insufficient disk space: need at least %d bytes, have %d", total+diskSafetyMarginBytes, available)
	}

	extracted := make([]ExtractedFile, 0, len(checked))
	for _, item := range checked {
		destination, joinErr := secureJoin(destinationRoot, item.relative)
		if joinErr != nil {
			return nil, 0, joinErr
		}
		if item.file.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return nil, 0, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return nil, 0, err
		}
		reader, err := item.file.Open()
		if err != nil {
			return nil, 0, err
		}
		mode := os.FileMode(0o644)
		if item.file.Mode()&0o111 != 0 {
			mode = 0o755
		}
		output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			_ = reader.Close()
			return nil, 0, err
		}
		written, copyErr := io.Copy(output, io.LimitReader(reader, int64(item.file.UncompressedSize64)+1))
		closeErr := output.Close()
		readerErr := reader.Close()
		if copyErr != nil {
			return nil, 0, copyErr
		}
		if closeErr != nil {
			return nil, 0, closeErr
		}
		if readerErr != nil {
			return nil, 0, readerErr
		}
		if uint64(written) != item.file.UncompressedSize64 {
			return nil, 0, fmt.Errorf("ZIP entry %q size mismatch", item.file.Name)
		}
		extractor.seen[strings.ToLower(item.relative)] = item.relative
		extracted = append(extracted, ExtractedFile{RelativePath: item.relative, SourcePath: destination, Size: uint64(written)})
	}
	return extracted, total, nil
}
