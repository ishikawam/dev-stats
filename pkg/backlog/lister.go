package backlog

import (
	"dev-stats/pkg/common"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Project represents a Backlog project
type Project struct {
	ID         int    `json:"id"`
	ProjectKey string `json:"projectKey"`
	Name       string `json:"name"`
	Archived   bool   `json:"archived"`
}

// ProjectMember represents a user in a project
type ProjectMember struct {
	ID       int    `json:"id"`
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	RoleType int    `json:"roleType"`
}

// ProjectWithMembers represents a project with its members
type ProjectWithMembers struct {
	Project
	Members []ProjectMember `json:"members"`
}

// ProfileCache represents cached data for a profile
type ProfileCache struct {
	Profile  string                `json:"profile"`
	Host     string                `json:"host"`
	CachedAt time.Time             `json:"cached_at"`
	Projects []ProjectWithMembers  `json:"projects"`
}

// ListProjects lists all projects in the Backlog space
func (b *BacklogAnalyzer) ListProjects(writer io.Writer) error {
	if b.profile.APIKey == "" {
		return common.NewError("BACKLOG_API_KEY environment variable is required")
	}
	if b.profile.Host == "" {
		return common.NewError("BACKLOG_HOST environment variable is required")
	}

	params := url.Values{}
	params.Set("apiKey", b.profile.APIKey)

	apiURL := fmt.Sprintf("%s/api/v2/projects?%s", b.profile.GetBaseURL(), params.Encode())

	body, err := b.client.Get(apiURL, nil)
	if err != nil {
		return common.WrapError(err, "failed to get projects")
	}

	var projects []Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return common.WrapError(err, "failed to parse projects response")
	}

	// Sort projects by ID
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ID < projects[j].ID
	})

	fmt.Fprintf(writer, "\n=== Backlog Projects (Host: %s) ===\n\n", b.profile.Host)
	fmt.Fprintf(writer, "%-12s %-10s %-40s %s\n", "ID", "Key", "Name", "Archived")
	fmt.Fprintf(writer, "%s\n", "--------------------------------------------------------------------------------")

	for _, project := range projects {
		archived := ""
		if project.Archived {
			archived = "✓"
		}
		fmt.Fprintf(writer, "%-12d %-10s %-40s %s\n",
			project.ID,
			project.ProjectKey,
			truncate(project.Name, 40),
			archived)
	}

	fmt.Fprintf(writer, "\nTotal projects: %d\n", len(projects))

	return nil
}

// ListProjectMembers lists all members of a specific project
func (b *BacklogAnalyzer) ListProjectMembers(projectID string, writer io.Writer) error {
	if b.profile.APIKey == "" {
		return common.NewError("BACKLOG_API_KEY environment variable is required")
	}
	if b.profile.Host == "" {
		return common.NewError("BACKLOG_HOST environment variable is required")
	}
	if projectID == "" {
		return common.NewError("Project ID is required")
	}

	params := url.Values{}
	params.Set("apiKey", b.profile.APIKey)

	apiURL := fmt.Sprintf("%s/api/v2/projects/%s/users?%s",
		b.profile.GetBaseURL(), projectID, params.Encode())

	body, err := b.client.Get(apiURL, nil)
	if err != nil {
		return common.WrapError(err, "failed to get project members")
	}

	var members []ProjectMember
	if err := json.Unmarshal(body, &members); err != nil {
		return common.WrapError(err, "failed to parse project members response")
	}

	// Sort members by ID
	sort.Slice(members, func(i, j int) bool {
		return members[i].ID < members[j].ID
	})

	fmt.Fprintf(writer, "\n=== Project Members (Project ID: %s) ===\n\n", projectID)
	fmt.Fprintf(writer, "%-12s %-20s %-40s %s\n", "ID", "User ID", "Name", "Role")
	fmt.Fprintf(writer, "%s\n", "--------------------------------------------------------------------------------")

	for _, member := range members {
		roleType := getRoleTypeName(member.RoleType)
		userID := member.UserID
		if userID == "" {
			userID = "-"
		}
		fmt.Fprintf(writer, "%-12d %-20s %-40s %s\n",
			member.ID,
			truncate(userID, 20),
			truncate(member.Name, 40),
			roleType)
	}

	fmt.Fprintf(writer, "\nTotal members: %d\n", len(members))

	return nil
}

// ListAllProjectsAndMembers lists all projects and their members
func (b *BacklogAnalyzer) ListAllProjectsAndMembers(writer io.Writer) error {
	if err := b.ListProjects(writer); err != nil {
		return err
	}

	// Get projects to list members
	params := url.Values{}
	params.Set("apiKey", b.profile.APIKey)
	apiURL := fmt.Sprintf("%s/api/v2/projects?%s", b.profile.GetBaseURL(), params.Encode())

	body, err := b.client.Get(apiURL, nil)
	if err != nil {
		return common.WrapError(err, "failed to get projects")
	}

	var projects []Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return common.WrapError(err, "failed to parse projects response")
	}

	// List members for each project
	for _, project := range projects {
		if !project.Archived {
			fmt.Fprintf(writer, "\n")
			if err := b.ListProjectMembers(fmt.Sprintf("%d", project.ID), writer); err != nil {
				fmt.Fprintf(writer, "Warning: Failed to get members for project %s: %v\n", project.Name, err)
			}
		}
	}

	return nil
}

// getCachePath returns the cache file path for a profile
func getCachePath(profileName string) string {
	cacheDir := ".backlog-cache"
	return filepath.Join(cacheDir, fmt.Sprintf("%s.json", profileName))
}

// loadCache loads cached data for a profile
func (b *BacklogAnalyzer) loadCache() (*ProfileCache, error) {
	cachePath := getCachePath(b.profile.Name)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var cache ProfileCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveCache saves data to cache
func (b *BacklogAnalyzer) saveCache(cache *ProfileCache) error {
	cacheDir := ".backlog-cache"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cachePath := getCachePath(b.profile.Name)
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// ListAllProjectsAndMembersWithCache lists all projects and members with caching
func (b *BacklogAnalyzer) ListAllProjectsAndMembersWithCache(writer io.Writer, forceRefresh bool) error {
	// Try to load from cache first
	if !forceRefresh {
		if cache, err := b.loadCache(); err == nil {
			fmt.Fprintf(writer, "\n📦 Using cached data (cached at: %s)\n", cache.CachedAt.Format("2006-01-02 15:04:05"))
			return b.displayCachedData(cache, writer)
		}
	}

	// Fetch fresh data from API
	fmt.Fprintf(writer, "\n🔄 Fetching fresh data from Backlog API...\n")

	// Get projects
	params := url.Values{}
	params.Set("apiKey", b.profile.APIKey)
	apiURL := fmt.Sprintf("%s/api/v2/projects?%s", b.profile.GetBaseURL(), params.Encode())

	body, err := b.client.Get(apiURL, nil)
	if err != nil {
		return common.WrapError(err, "failed to get projects")
	}

	var projects []Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return common.WrapError(err, "failed to parse projects response")
	}

	// Get members for each project
	var projectsWithMembers []ProjectWithMembers
	for _, project := range projects {
		if project.Archived {
			continue
		}

		members, err := b.getProjectMembersInternal(fmt.Sprintf("%d", project.ID))
		if err != nil {
			fmt.Fprintf(writer, "Warning: Failed to get members for project %s: %v\n", project.Name, err)
			continue
		}

		projectsWithMembers = append(projectsWithMembers, ProjectWithMembers{
			Project: project,
			Members: members,
		})
	}

	// Save to cache
	cache := &ProfileCache{
		Profile:  b.profile.Name,
		Host:     b.profile.Host,
		CachedAt: time.Now(),
		Projects: projectsWithMembers,
	}

	if err := b.saveCache(cache); err != nil {
		fmt.Fprintf(writer, "Warning: Failed to save cache: %v\n", err)
	} else {
		fmt.Fprintf(writer, "✓ Data cached successfully\n")
	}

	return b.displayCachedData(cache, writer)
}

// getProjectMembersInternal gets members of a project (internal use)
func (b *BacklogAnalyzer) getProjectMembersInternal(projectID string) ([]ProjectMember, error) {
	params := url.Values{}
	params.Set("apiKey", b.profile.APIKey)

	apiURL := fmt.Sprintf("%s/api/v2/projects/%s/users?%s",
		b.profile.GetBaseURL(), projectID, params.Encode())

	body, err := b.client.Get(apiURL, nil)
	if err != nil {
		return nil, err
	}

	var members []ProjectMember
	if err := json.Unmarshal(body, &members); err != nil {
		return nil, err
	}

	sort.Slice(members, func(i, j int) bool {
		return members[i].ID < members[j].ID
	})

	return members, nil
}

// displayCachedData displays cached data
func (b *BacklogAnalyzer) displayCachedData(cache *ProfileCache, writer io.Writer) error {
	// Display projects summary
	fmt.Fprintf(writer, "\n=== Backlog Projects (Host: %s) ===\n\n", cache.Host)
	fmt.Fprintf(writer, "%-12s %-10s %-40s %s\n", "ID", "Key", "Name", "Members")
	fmt.Fprintf(writer, "%s\n", "--------------------------------------------------------------------------------")

	for _, project := range cache.Projects {
		fmt.Fprintf(writer, "%-12d %-10s %-40s %d\n",
			project.ID,
			project.ProjectKey,
			truncate(project.Name, 40),
			len(project.Members))
	}

	fmt.Fprintf(writer, "\nTotal projects: %d\n", len(cache.Projects))

	// Display members for each project
	for _, project := range cache.Projects {
		fmt.Fprintf(writer, "\n=== Project Members (Project ID: %d - %s) ===\n\n", project.ID, project.Name)
		fmt.Fprintf(writer, "%-12s %-20s %-40s %s\n", "ID", "User ID", "Name", "Role")
		fmt.Fprintf(writer, "%s\n", "--------------------------------------------------------------------------------")

		for _, member := range project.Members {
			roleType := getRoleTypeName(member.RoleType)
			userID := member.UserID
			if userID == "" {
				userID = "-"
			}
			fmt.Fprintf(writer, "%-12d %-20s %-40s %s\n",
				member.ID,
				truncate(userID, 20),
				truncate(member.Name, 40),
				roleType)
		}

		fmt.Fprintf(writer, "\nTotal members: %d\n", len(project.Members))
	}

	return nil
}

// ClearCache clears the cache for a profile
func ClearCache(profileName string) error {
	cachePath := getCachePath(profileName)
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// getRoleTypeName converts role type integer to readable name
func getRoleTypeName(roleType int) string {
	switch roleType {
	case 1:
		return "Administrator"
	case 2:
		return "Member"
	case 3:
		return "Reporter"
	case 4:
		return "Viewer"
	default:
		return fmt.Sprintf("Unknown(%d)", roleType)
	}
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
