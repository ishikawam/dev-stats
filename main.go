package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
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

func main() {
	// クエリの作成
	username := os.Getenv("GITHUB_USERNAME")
	token := os.Getenv("GITHUB_TOKEN")

	query := fmt.Sprintf("involves:%s type:pr created:%s..%s", username, startDate, endDate)
	fullURL := fmt.Sprintf("%s?q=%s", githubAPIURL, url.QueryEscape(query))

	// リクエストの作成
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("User-Agent", "Go-GitHub-PR-Extractor")

	// API 呼び出し
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("GitHub API returned status: %s", resp.Status)
	}

	// レスポンスの読み込み
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	// JSON パース
	var prResponse PRResponse
	err = json.Unmarshal(body, &prResponse)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	// 結果の出力
	if len(prResponse.Items) == 0 {
		fmt.Println("No PRs found for the specified period.")
		return
	}

	fmt.Printf("Pull Requests you were involved in from %s to %s:\n", startDate, endDate)
	for _, pr := range prResponse.Items {
		fmt.Printf("- Title: %s\n", pr.Title)
		fmt.Printf("  URL: %s\n", pr.URL)
		fmt.Printf("  Created At: %s\n", pr.CreatedAt)
		fmt.Printf("  Repository: %s\n", pr.RepoURL)
		fmt.Println()
	}
}
