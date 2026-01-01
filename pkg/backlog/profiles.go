package backlog

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// BacklogProfile represents a Backlog environment configuration
type BacklogProfile struct {
	Name      string
	APIKey    string
	Host      string // e.g., "mycompany.backlog.com" or "projectspace.backlog.jp"
	UserID    string
	ProjectID string
}

// GetBaseURL returns the base URL for this profile
func (p *BacklogProfile) GetBaseURL() string {
	return fmt.Sprintf("https://%s", p.Host)
}

// IsComplete returns true if all required fields are set
func (p *BacklogProfile) IsComplete() bool {
	return p.APIKey != "" && p.Host != ""
}

// IsAnalysisReady returns true if profile is ready for analysis (has user and project IDs)
func (p *BacklogProfile) IsAnalysisReady() bool {
	return p.IsComplete() && p.UserID != "" && p.ProjectID != ""
}

// LoadBacklogProfiles loads all Backlog profiles from environment variables
// Profiles are defined with pattern: BACKLOG_<PROFILE_NAME>_<SETTING>
func LoadBacklogProfiles() []BacklogProfile {
	profileMap := make(map[string]*BacklogProfile)

	// Scan all environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Check if it's a Backlog profile variable
		if !strings.HasPrefix(key, "BACKLOG_") {
			continue
		}

		// Skip non-profile variables
		if key == "BACKLOG_PROFILE" {
			continue
		}

		// Parse the key: BACKLOG_<PROFILE>_<SETTING>
		keyParts := strings.Split(key, "_")
		if len(keyParts) < 3 {
			continue
		}

		profileName := keyParts[1]
		setting := strings.Join(keyParts[2:], "_")

		// Get or create profile
		if _, exists := profileMap[profileName]; !exists {
			profileMap[profileName] = &BacklogProfile{
				Name: profileName,
			}
		}

		profile := profileMap[profileName]

		// Set the appropriate field
		switch setting {
		case "API_KEY":
			profile.APIKey = value
		case "HOST":
			profile.Host = value
		case "USER_ID":
			profile.UserID = value
		case "PROJECT_ID":
			profile.ProjectID = value
		}
	}

	// Convert map to slice and sort by name
	profiles := make([]BacklogProfile, 0, len(profileMap))
	for _, profile := range profileMap {
		if profile.IsComplete() {
			profiles = append(profiles, *profile)
		}
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return profiles
}

// GetProfileByName returns a specific profile by name
func GetProfileByName(name string) (*BacklogProfile, error) {
	profiles := LoadBacklogProfiles()
	for _, profile := range profiles {
		if strings.EqualFold(profile.Name, name) {
			return &profile, nil
		}
	}
	return nil, fmt.Errorf("profile '%s' not found", name)
}
