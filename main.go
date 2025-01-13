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

func main() {
	username := os.Getenv("GITHUB_USERNAME")
	token := os.Getenv("GITHUB_TOKEN")
	if username == "" || token == "" {
		log.Fatalf("Environment variables GITHUB_USERNAME and GITHUB_TOKEN must be set.")
	}

	// クエリ1: created の期間
	queryCreated := fmt.Sprintf("involves:%s type:pr created:%s..%s", username, startDate, endDate)
	createdPRs := fetchPRs(queryCreated, token)

	// クエリ2: updated の期間
	queryUpdated := fmt.Sprintf("involves:%s type:pr updated:%s..%s", username, startDate, endDate)
	updatedPRs := fetchPRs(queryUpdated, token)

	// 結果を統合（重複を排除するにはマップを使用）
	prMap := make(map[string]PR)
	for _, pr := range createdPRs {
		prMap[pr.URL] = pr
	}
	for _, pr := range updatedPRs {
		prMap[pr.URL] = pr
	}

	// マップからスライスに変換
	allPRs := make([]PR, 0, len(prMap))
	for _, pr := range prMap {
		allPRs = append(allPRs, pr)
	}

	// リポジトリごとおよび組織ごとに集計
	repoCount := make(map[string]int)
	orgCount := make(map[string]int)
	for _, pr := range allPRs {
		// `RepoURL` から `organization/repository` を抽出
		repoParts := strings.Split(pr.RepoURL, "/")
		if len(repoParts) >= 2 {
			orgRepo := fmt.Sprintf("%s/%s", repoParts[len(repoParts)-2], repoParts[len(repoParts)-1])
			repoCount[orgRepo]++

			// 組織名を抽出して集計
			org := repoParts[len(repoParts)-2]
			orgCount[org]++
		}
	}

	// `organization/repository` をソート
	sortedRepos := make([]string, 0, len(repoCount))
	for repo := range repoCount {
		sortedRepos = append(sortedRepos, repo)
	}
	sort.Strings(sortedRepos)

	// 結果を出力
	if len(allPRs) == 0 {
		fmt.Println("No PRs found for the specified criteria.")
		return
	}

	fmt.Printf("Pull Requests you were involved in (created or updated) from %s to %s:\n", startDate, endDate)
	for _, pr := range allPRs {
		fmt.Printf("- Title: %s\n", pr.Title)
		fmt.Printf("  URL: %s\n", pr.URL)
		fmt.Printf("  Created At: %s\n", pr.CreatedAt)
		fmt.Printf("  Repository: %s\n", pr.RepoURL)
		fmt.Println()
	}

	fmt.Printf("\nTotal PRs: %d\n", len(allPRs))

	// 組織ごとの PR 数を出力
	fmt.Println("\nPR count per organization:")
	for org, count := range orgCount {
		fmt.Printf("- %s: %d\n", org, count)
	}

	// ソート済みのリポジトリごとの PR 数を出力
	fmt.Println("\nPR count per repository (sorted by organization/repository):")
	for _, repo := range sortedRepos {
		fmt.Printf("- %s: %d\n", repo, repoCount[repo])
	}
}
