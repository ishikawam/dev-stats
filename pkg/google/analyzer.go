package google

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"dev-stats/pkg/common"

	"google.golang.org/api/drive/v3"
)

const driveFileFields = "nextPageToken, files(id, name, mimeType, modifiedTime, createdTime, owners, lastModifyingUser, webViewLink, size)"

const (
	mimeDoc   = "application/vnd.google-apps.document"
	mimeSlide = "application/vnd.google-apps.presentation"
	mimeSheet = "application/vnd.google-apps.spreadsheet"
)

// fileTypeLabel returns a short label for the MIME type.
func fileTypeLabel(mimeType string) string {
	switch mimeType {
	case mimeDoc:
		return "Doc"
	case mimeSlide:
		return "Slide"
	case mimeSheet:
		return "Sheet"
	default:
		return "File"
	}
}

// GDocsAnalyzer implements the Analyzer interface for Google Workspace files (Docs, Slides, Sheets).
type GDocsAnalyzer struct{}

// GDocsFile represents a single Google Docs/Drive file.
type GDocsFile struct {
	ID             string
	Name           string
	MimeType       string
	CreatedTime    time.Time
	ModifiedTime   time.Time
	LastModifiedBy string
	OwnerEmail     string
	WebViewLink    string
}

// NewGDocsAnalyzer creates a new GDocsAnalyzer.
func NewGDocsAnalyzer() *GDocsAnalyzer {
	return &GDocsAnalyzer{}
}

// GetName returns the analyzer name.
func (g *GDocsAnalyzer) GetName() string {
	return "Google Workspace"
}

// Analyze fetches Google Workspace files updated within config date range and prints results.
func (g *GDocsAnalyzer) Analyze(config *common.Config, writer io.Writer) (*common.AnalysisResult, error) {
	ctx := context.Background()

	client, err := getHTTPClient(ctx)
	if err != nil {
		return nil, common.WrapError(err, "failed to authenticate with Google")
	}

	svc, err := newDriveService(ctx, client)
	if err != nil {
		return nil, common.WrapError(err, "failed to create Drive service")
	}

	fmt.Fprintf(writer, "Searching Google Workspace files (Docs/Slides/Sheets) modified between %s and %s...\n",
		config.StartDate.Format("2006-01-02"),
		config.EndDate.Format("2006-01-02"),
	)

	me, err := getMyUserInfo(svc)
	if err != nil {
		return nil, common.WrapError(err, "failed to get user info")
	}
	fmt.Fprintf(writer, "Authenticated as: %s (%s)\n", me.DisplayName, me.EmailAddress)

	files, err := listModifiedFiles(svc, config.StartDate, config.EndDate, writer)
	if err != nil {
		return nil, common.WrapError(err, "failed to list Drive files")
	}

	relatedKeywords := relatedKeywordsFromEnv()

	created, updated, related, excluded := categorizeFiles(files, config.StartDate, config.EndDate, me, relatedKeywords)

	printFileResults(writer, created, updated, related, excluded, config.StartDate, config.EndDate)

	result := &common.AnalysisResult{
		AnalyzerName: g.GetName(),
		StartDate:    config.StartDate,
		EndDate:      config.EndDate,
		Summary: map[string]interface{}{
			"Files created":  len(created),
			"Files updated":  len(updated),
			"Files related":  len(related),
			"Files excluded": len(excluded),
			"Total files":    len(files),
		},
		Details: map[string]interface{}{
			"created_files":  created,
			"updated_files":  updated,
			"related_files":  related,
			"excluded_files": excluded,
		},
	}

	result.PrintSummary(writer)
	return result, nil
}

// listModifiedFiles fetches Google Workspace files modified in the given range.
func listModifiedFiles(svc *drive.Service, start, end time.Time, writer io.Writer) ([]GDocsFile, error) {
	startStr := start.Format(time.RFC3339)
	endStr := end.AddDate(0, 0, 1).Format(time.RFC3339)

	query := fmt.Sprintf(
		"(mimeType='%s' or mimeType='%s' or mimeType='%s') and modifiedTime >= '%s' and modifiedTime < '%s' and trashed = false and ('me' in owners or 'me' in writers)",
		mimeDoc, mimeSlide, mimeSheet, startStr, endStr,
	)

	var allFiles []GDocsFile
	pageToken := ""
	page := 0

	for {
		page++
		call := svc.Files.List().
			Q(query).
			Fields(driveFileFields).
			OrderBy("modifiedTime desc").
			PageSize(100)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("Drive API list error (page %d): %w", page, err)
		}

		fmt.Fprintf(writer, "  Page %d: %d files found\n", page, len(result.Files))

		for _, f := range result.Files {
			file, err := parseFile(f)
			if err != nil {
				fmt.Fprintf(writer, "  Warning: failed to parse file %s: %v\n", f.Id, err)
				continue
			}
			allFiles = append(allFiles, file)
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	fmt.Fprintf(writer, "Total files found: %d\n", len(allFiles))
	return allFiles, nil
}

// parseFile converts a Drive API file to GDocsFile.
func parseFile(f *drive.File) (GDocsFile, error) {
	created, err := time.Parse(time.RFC3339, f.CreatedTime)
	if err != nil {
		return GDocsFile{}, fmt.Errorf("invalid createdTime: %w", err)
	}
	modified, err := time.Parse(time.RFC3339, f.ModifiedTime)
	if err != nil {
		return GDocsFile{}, fmt.Errorf("invalid modifiedTime: %w", err)
	}

	lastModifiedBy := ""
	if f.LastModifyingUser != nil {
		lastModifiedBy = f.LastModifyingUser.DisplayName
	}

	ownerEmail := ""
	if len(f.Owners) > 0 {
		ownerEmail = f.Owners[0].EmailAddress
	}

	return GDocsFile{
		ID:             f.Id,
		Name:           f.Name,
		MimeType:       f.MimeType,
		CreatedTime:    created,
		ModifiedTime:   modified,
		LastModifiedBy: lastModifiedBy,
		OwnerEmail:     ownerEmail,
		WebViewLink:    f.WebViewLink,
	}, nil
}

// relatedKeywordsFromEnv reads GOOGLE_DOCS_RELATED_NAMES from environment.
func relatedKeywordsFromEnv() []string {
	val := os.Getenv("GOOGLE_DOCS_RELATED_NAMES")
	if val == "" {
		return nil
	}
	var keywords []string
	for _, k := range strings.Split(val, ",") {
		if k = strings.TrimSpace(k); k != "" {
			keywords = append(keywords, k)
		}
	}
	return keywords
}

// titleMatchesKeywords returns true if the file name contains any of the keywords (case-insensitive).
func titleMatchesKeywords(name string, keywords []string) bool {
	lower := strings.ToLower(name)
	for _, k := range keywords {
		if strings.Contains(lower, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

// categorizeFiles separates files into created, updated, related, and excluded within the range.
func categorizeFiles(files []GDocsFile, start, end time.Time, me myUserInfo, relatedKeywords []string) (created, updated, related, excluded []GDocsFile) {
	endInclusive := end.AddDate(0, 0, 1)
	for _, f := range files {
		inRange := f.CreatedTime.After(start) && f.CreatedTime.Before(endInclusive)
		isOwner := f.OwnerEmail == me.EmailAddress
		isLastModifier := f.LastModifiedBy == me.DisplayName

		if inRange && isOwner {
			created = append(created, f)
		} else if isLastModifier {
			updated = append(updated, f)
		} else if len(relatedKeywords) > 0 && titleMatchesKeywords(f.Name, relatedKeywords) {
			related = append(related, f)
		} else {
			excluded = append(excluded, f)
		}
	}
	return
}

// printFileResults prints the file listing to writer.
func printFileResults(writer io.Writer, created, updated, related, excluded []GDocsFile, start, end time.Time) {
	fmt.Fprintf(writer, "\nGoogle Workspace activity from %s to %s:\n",
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)

	for _, s := range [][]GDocsFile{created, updated, related, excluded} {
		sort.Slice(s, func(i, j int) bool {
			return s[i].ModifiedTime.Before(s[j].ModifiedTime)
		})
	}

	fmt.Fprintf(writer, "\nFiles you created (%d):\n", len(created))
	for _, f := range created {
		fmt.Fprintf(writer, "- [%s] %s: %s\n", fileTypeLabel(f.MimeType), f.ModifiedTime.Format("2006-01-02 15:04"), f.Name)
		fmt.Fprintf(writer, "  URL: %s\n\n", f.WebViewLink)
	}

	fmt.Fprintf(writer, "Files updated (%d):\n", len(updated))
	for _, f := range updated {
		modifier := f.LastModifiedBy
		if modifier == "" {
			modifier = "-"
		}
		fmt.Fprintf(writer, "- [%s] %s: %s\n", fileTypeLabel(f.MimeType), f.ModifiedTime.Format("2006-01-02 15:04"), f.Name)
		fmt.Fprintf(writer, "  Modified by: %s\n", modifier)
		fmt.Fprintf(writer, "  URL: %s\n\n", f.WebViewLink)
	}

	if len(related) > 0 {
		fmt.Fprintf(writer, "\nFiles related (title matches GOOGLE_DOCS_RELATED_NAMES) (%d):\n", len(related))
		for _, f := range related {
			modifier := f.LastModifiedBy
			if modifier == "" {
				modifier = "-"
			}
			fmt.Fprintf(writer, "- [%s] %s: %s\n", fileTypeLabel(f.MimeType), f.ModifiedTime.Format("2006-01-02 15:04"), f.Name)
			fmt.Fprintf(writer, "  Owner: %s / Last modified by: %s\n", f.OwnerEmail, modifier)
			fmt.Fprintf(writer, "  URL: %s\n\n", f.WebViewLink)
		}
	}

	fmt.Fprintf(writer, "\nFiles excluded (%d):\n", len(excluded))
	for _, f := range excluded {
		modifier := f.LastModifiedBy
		if modifier == "" {
			modifier = "-"
		}
		fmt.Fprintf(writer, "- [%s] %s: %s\n", fileTypeLabel(f.MimeType), f.ModifiedTime.Format("2006-01-02 15:04"), f.Name)
		fmt.Fprintf(writer, "  Owner: %s / Last modified by: %s\n", f.OwnerEmail, modifier)
		fmt.Fprintf(writer, "  URL: %s\n\n", f.WebViewLink)
	}
}
