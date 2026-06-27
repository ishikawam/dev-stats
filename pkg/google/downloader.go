package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dev-stats/pkg/common"

	"google.golang.org/api/drive/v3"
)

const (
	mimeMarkdown  = "text/markdown"
	mimePlainText = "text/plain"
	mimeCSV       = "text/csv"
)

// GDocsDownloader downloads Google Docs files found in the given date range.
type GDocsDownloader struct{}

// NewGDocsDownloader creates a new GDocsDownloader.
func NewGDocsDownloader() *GDocsDownloader {
	return &GDocsDownloader{}
}

// DownloadAll fetches Google Workspace files related to the user in the given range and saves them locally.
// Output directory: output/YYYY-MM-DD_to_YYYY-MM-DD/google/
func (d *GDocsDownloader) DownloadAll(start, end time.Time, writer io.Writer) error {
	ctx := context.Background()

	client, err := getHTTPClient(ctx)
	if err != nil {
		return common.WrapError(err, "failed to authenticate with Google")
	}

	driveSvc, err := newDriveService(ctx, client)
	if err != nil {
		return common.WrapError(err, "failed to create Drive service")
	}

	me, err := getMyUserInfo(driveSvc)
	if err != nil {
		return common.WrapError(err, "failed to get user info")
	}

	allFiles, err := listModifiedFiles(driveSvc, start, end, writer)
	if err != nil {
		return common.WrapError(err, "failed to list Drive files")
	}

	relatedKeywords := relatedKeywordsFromEnv()
	created, updated, related, excluded := categorizeFiles(allFiles, start, end, me, relatedKeywords)

	outDir := fmt.Sprintf("output/%s_to_%s/google",
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return common.WrapError(err, "failed to create output directory")
	}

	var revisionRelated []GDocsFile
	if os.Getenv("GOOGLE_DOCS_CHECK_REVISIONS") == "true" && len(excluded) > 0 {
		cache := loadRevisionCache(outDir)
		cacheUpdated := false
		checkCount := 0

		for i, f := range excluded {
			if entry, ok := cache[f.ID]; ok && entry.ModifiedTime.Equal(f.ModifiedTime) {
				if entry.HasMyRevision {
					revisionRelated = append(revisionRelated, f)
				}
				continue
			}

			checkCount++
			fmt.Fprintf(writer, "  Checking (%d/%d): %s\r", i+1, len(excluded), f.Name)
			found, err := hasMyRevision(driveSvc, f.ID, me.EmailAddress)
			if err != nil {
				continue
			}
			cache[f.ID] = revisionCacheEntry{HasMyRevision: found, ModifiedTime: f.ModifiedTime}
			cacheUpdated = true
			if found {
				revisionRelated = append(revisionRelated, f)
			}
			time.Sleep(200 * time.Millisecond)
		}

		if cacheUpdated {
			saveRevisionCache(outDir, cache)
		}
		if checkCount > 0 {
			fmt.Fprintf(writer, "\nChecked %d files, found %d with your revision history\n", checkCount, len(revisionRelated))
		} else {
			fmt.Fprintf(writer, "\nRevision cache up to date, found %d files with your revision history\n", len(revisionRelated))
		}
	}

	var files []GDocsFile
	files = append(files, created...)
	files = append(files, updated...)
	files = append(files, related...)
	files = append(files, revisionRelated...)

	if len(files) == 0 {
		fmt.Fprintln(writer, "No relevant files found in the specified date range.")
		return nil
	}

	fmt.Fprintf(writer, "Downloading %d files (created: %d, updated: %d, related: %d, revision: %d) to: %s\n",
		len(files), len(created), len(updated), len(related), len(revisionRelated), outDir)

	subDirs := map[string]string{
		mimeDoc:   filepath.Join(outDir, "docs"),
		mimeSlide: filepath.Join(outDir, "slides"),
		mimeSheet: filepath.Join(outDir, "sheets"),
	}
	for _, dir := range subDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return common.WrapError(err, "failed to create subdirectory")
		}
	}

	successCount := 0
	skipCount := 0
	for i, f := range files {
		subDir := subDirs[f.MimeType]
		if subDir == "" {
			subDir = outDir
		}

		ext := ".md"
		if f.MimeType == mimeSheet {
			ext = ".csv"
		}
		fileName := sanitizeFileName(f.Name) + ext
		filePath := filepath.Join(subDir, fileName)

		if info, err := os.Stat(filePath); err == nil {
			if !info.ModTime().Before(f.ModifiedTime) {
				fmt.Fprintf(writer, "  Skipping  (%d/%d): %s (up to date)\n", i+1, len(files), f.Name)
				skipCount++
				continue
			}
		}

		fmt.Fprintf(writer, "  Downloading (%d/%d): %s\n", i+1, len(files), f.Name)

		if err := d.downloadFile(driveSvc, f, subDir, writer); err != nil {
			fmt.Fprintf(writer, "    Warning: failed to download %s: %v\n", f.Name, err)
			continue
		}

		successCount++
		fmt.Fprintf(writer, "    ✓ Done\n")

		time.Sleep(300 * time.Millisecond)
	}

	fmt.Fprintf(writer, "\nDownload completed: %d downloaded, %d skipped (up to date), %d total\n",
		successCount, skipCount, len(files))
	return nil
}

// downloadFile exports a single Google Workspace file and writes it to outDir.
func (d *GDocsDownloader) downloadFile(svc *drive.Service, f GDocsFile, outDir string, writer io.Writer) error {
	var content, fileName string

	switch f.MimeType {
	case mimeSheet:
		raw, err := exportFileAs(svc, f.ID, mimeCSV)
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fileName = sanitizeFileName(f.Name) + ".csv"
		content = raw
	case mimeSlide:
		raw, err := exportFileAs(svc, f.ID, mimePlainText)
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fileName = sanitizeFileName(f.Name) + ".md"
		content = buildMarkdownWrapper(f, raw)
	default: // Docs
		raw, usedMIME, err := exportFile(svc, f.ID)
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fileName = sanitizeFileName(f.Name) + ".md"
		if usedMIME != mimeMarkdown {
			content = buildMarkdownWrapper(f, raw)
		} else {
			content = buildMetadataHeader(f) + raw
		}
	}

	filePath := filepath.Join(outDir, fileName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}

type revisionCacheEntry struct {
	HasMyRevision bool      `json:"has_my_revision"`
	ModifiedTime  time.Time `json:"modified_time"`
}

type revisionCache map[string]revisionCacheEntry

func revisionCachePath(outDir string) string {
	return filepath.Join(outDir, ".cache", "revision-cache.json")
}

func loadRevisionCache(outDir string) revisionCache {
	cache := make(revisionCache)
	data, err := os.ReadFile(revisionCachePath(outDir))
	if err != nil {
		return cache
	}
	_ = json.Unmarshal(data, &cache)
	return cache
}

func saveRevisionCache(outDir string, cache revisionCache) {
	cacheDir := filepath.Join(outDir, ".cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(revisionCachePath(outDir), data, 0644)
}

// hasMyRevision returns true if the file has a revision made by the given email address.
func hasMyRevision(svc *drive.Service, fileID, myEmail string) (bool, error) {
	revisions, err := svc.Revisions.List(fileID).Fields("revisions(lastModifyingUser)").Do()
	if err != nil {
		return false, err
	}
	for _, rev := range revisions.Revisions {
		if rev.LastModifyingUser != nil && rev.LastModifyingUser.EmailAddress == myEmail {
			return true, nil
		}
	}
	return false, nil
}

// exportFileAs exports a file in the specified MIME type.
func exportFileAs(svc *drive.Service, fileID, mime string) (string, error) {
	res, err := svc.Files.Export(fileID, mime).Download()
	if err != nil {
		return "", fmt.Errorf("failed to export as %s: %w", mime, err)
	}
	defer res.Body.Close()
	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	return string(buf), nil
}

// exportFile attempts Markdown export, falling back to plain text.
func exportFile(svc *drive.Service, fileID string) (string, string, error) {
	res, err := svc.Files.Export(fileID, mimeMarkdown).Download()
	if err == nil {
		defer res.Body.Close()
		buf, err := io.ReadAll(res.Body)
		if err != nil {
			return "", "", fmt.Errorf("failed to read markdown response: %w", err)
		}
		return string(buf), mimeMarkdown, nil
	}

	res, err = svc.Files.Export(fileID, mimePlainText).Download()
	if err != nil {
		return "", "", fmt.Errorf("failed to export as plain text: %w", err)
	}
	defer res.Body.Close()
	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read plain text response: %w", err)
	}
	return string(buf), mimePlainText, nil
}

// buildMetadataHeader returns a Markdown metadata header for the file.
func buildMetadataHeader(f GDocsFile) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", f.Name))
	sb.WriteString(fmt.Sprintf("**Created:** %s  \n", f.CreatedTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Last Modified:** %s  \n", f.ModifiedTime.Format("2006-01-02 15:04:05")))
	if f.LastModifiedBy != "" {
		sb.WriteString(fmt.Sprintf("**Modified by:** %s  \n", f.LastModifiedBy))
	}
	sb.WriteString(fmt.Sprintf("**URL:** %s  \n\n", f.WebViewLink))
	sb.WriteString("---\n\n")
	return sb.String()
}

// buildMarkdownWrapper wraps plain text content in a Markdown document.
func buildMarkdownWrapper(f GDocsFile, content string) string {
	var sb strings.Builder
	sb.WriteString(buildMetadataHeader(f))
	sb.WriteString("## Content\n\n")
	sb.WriteString(content)
	return sb.String()
}

// sanitizeFileName removes filesystem-incompatible characters from a filename.
func sanitizeFileName(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, ch := range invalid {
		name = strings.ReplaceAll(name, ch, "_")
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}
