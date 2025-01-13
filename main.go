package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

const (
	githubAPIURL = "https://api.github.com/search/issues"
	startDate    = "2024-07-01"
	endDate      = "2024-12-31"
)

type PR struct {
	Title     string `json:"title"`
	URL       string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	RepoURL   string `json:"repository_url"`
}

type PRResponse struct {
	Items []PR `json:"items"`
}

func fetchPRs(query string, token string) []PR {
	allPRs := []PR{}
	perPage := 100
	page := 1

	for {
		fullURL := fmt.Sprintf("%s?q=%s&per_page=%d&page=%d", githubAPIURL, url.QueryEscape(query), perPage, page)
		fmt.Printf("Fetching: %s\n", fullURL)

		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			log.Fatalf("Error creating request: %v", err)
		}
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("User-Agent", "Go-GitHub-PR-Extractor")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Error making request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("GitHub API returned status: %s", resp.Status)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading response body: %v", err)
		}

		var prResponse PRResponse
		err = json.Unmarshal(body, &prResponse)
		if err != nil {
			log.Fatalf("Error unmarshalling JSON: %v", err)
		}

		allPRs = append(allPRs, prResponse.Items...)

		if len(prResponse.Items) < perPage {
			break
		}
		page++
	}

	return allPRs
}

func extractOrganization(repoURL string) string {
	repoParts := strings.Split(repoURL, "/")
	if len(repoParts) >= 2 {
		return repoParts[len(repoParts)-2]
	}
	return ""
}

func extractRepo(repoURL string) string {
	repoParts := strings.Split(repoURL, "/")
	if len(repoParts) >= 2 {
		return fmt.Sprintf("%s/%s", repoParts[len(repoParts)-2], repoParts[len(repoParts)-1])
	}
	return ""
}

func main() {
	username := os.Getenv("GITHUB_USERNAME")
	token := os.Getenv("GITHUB_TOKEN")
	if username == "" || token == "" {
		log.Fatalf("Environment variables GITHUB_USERNAME and GITHUB_TOKEN must be set.")
	}

	// createdとmerged。involvesとauthor。updated, closedは含めない。

	// involves の PR を取得
	queryInvolvesCreated := fmt.Sprintf("involves:%s type:pr created:%s..%s", username, startDate, endDate)
	queryInvolvesMerged := fmt.Sprintf("involves:%s type:pr merged:%s..%s", username, startDate, endDate)
	involvesPRsCreated := fetchPRs(queryInvolvesCreated, token)
	involvesPRsMerged := fetchPRs(queryInvolvesMerged, token)

	// author の PR を取得
	queryAuthorCreated := fmt.Sprintf("author:%s type:pr created:%s..%s", username, startDate, endDate)
	queryAuthorMerged := fmt.Sprintf("author:%s type:pr merged:%s..%s", username, startDate, endDate)
	authorPRsCreated := fetchPRs(queryAuthorCreated, token)
	authorPRsMerged := fetchPRs(queryAuthorMerged, token)

	// 結果を統合
	involvesPRs := make(map[string]PR)
	for _, pr := range involvesPRsCreated {
		involvesPRs[pr.URL] = pr
	}
	for _, pr := range involvesPRsMerged {
		involvesPRs[pr.URL] = pr
	}

	authorPRs := make(map[string]PR)
	for _, pr := range authorPRsCreated {
		authorPRs[pr.URL] = pr
	}
	for _, pr := range authorPRsMerged {
		authorPRs[pr.URL] = pr
	}

	// 全 PR をリスト化
	allPRs := make([]PR, 0, len(involvesPRs))
	for _, pr := range involvesPRs {
		allPRs = append(allPRs, pr)
	}

	// URLでソート
	sort.Slice(allPRs, func(i, j int) bool {
		return allPRs[i].URL < allPRs[j].URL
	})

	// 集計
	orgRepoInvolvesCount := make(map[string]int)
	orgRepoAuthorCount := make(map[string]int)
	orgInvolvesCount := make(map[string]int)
	orgAuthorCount := make(map[string]int)

	for _, pr := range involvesPRs {
		org := extractOrganization(pr.RepoURL)
		repo := extractRepo(pr.RepoURL)
		orgRepoInvolvesCount[repo]++
		orgInvolvesCount[org]++
	}

	for _, pr := range authorPRs {
		org := extractOrganization(pr.RepoURL)
		repo := extractRepo(pr.RepoURL)
		orgRepoAuthorCount[repo]++
		orgAuthorCount[org]++
	}

	// 組織のソート
	sortedOrganizations := make([]string, 0, len(orgInvolvesCount))
	for org := range orgInvolvesCount {
		sortedOrganizations = append(sortedOrganizations, org)
	}
	sort.Strings(sortedOrganizations)

	// リポジトリのソート
	sortedRepos := make([]string, 0, len(orgRepoInvolvesCount))
	for repo := range orgRepoInvolvesCount {
		sortedRepos = append(sortedRepos, repo)
	}
	sort.Strings(sortedRepos)

	// 出力
	fmt.Printf("Pull Requests you were involved in (created or merged) from %s to %s:\n", startDate, endDate)
	for _, pr := range allPRs {
		fmt.Printf("- Title: %s\n", pr.Title)
		fmt.Printf("  URL: %s\n", pr.URL)
		fmt.Printf("  Created At: %s\n", pr.CreatedAt)
		fmt.Printf("  Repository: %s\n", pr.RepoURL)
		fmt.Println()
	}

	fmt.Printf("Pull Requests summary from %s to %s:\n", startDate, endDate)

	fmt.Printf("\nTotal PRs: %d\n", len(allPRs))
	fmt.Printf("\nTotal PRs (author): %d\n", len(authorPRs))
	fmt.Printf("Total PRs (involves): %d\n", len(involvesPRs))

	// 組織ごとの出力
	fmt.Println("\nPR count per organization (author/involves):")
	for _, org := range sortedOrganizations {
		authorCount := orgAuthorCount[org]
		involvesCount := orgInvolvesCount[org]
		fmt.Printf("- %s: %d (%d)\n", org, authorCount, involvesCount)
	}

	// リポジトリごとの出力
	fmt.Println("\nPR count per repository (author/involves):")
	for _, repo := range sortedRepos {
		authorCount := orgRepoAuthorCount[repo]
		involvesCount := orgRepoInvolvesCount[repo]
		fmt.Printf("- %s: %d (%d)\n", repo, authorCount, involvesCount)
	}
}
