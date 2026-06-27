package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// newOAuth2Config builds the OAuth2 config from environment variables.
// Uses localhost redirect URI; OOB flow is deprecated by Google.
func newOAuth2Config(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectURL,
		Scopes: []string{
			drive.DriveReadonlyScope,
			calendar.CalendarReadonlyScope,
		},
		Endpoint: google.Endpoint,
	}
}

// tokenFilePath returns the token cache file path.
func tokenFilePath() string {
	if path := os.Getenv("GOOGLE_TOKEN_FILE"); path != "" {
		return path
	}
	return "storage/google_token.json"
}

// loadToken loads a cached OAuth2 token from disk.
func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok oauth2.Token
	if err := json.NewDecoder(f).Decode(&tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

// saveToken persists an OAuth2 token to disk.
func saveToken(path string, tok *oauth2.Token) error {
	if err := os.MkdirAll("storage", 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tok)
}

// getHTTPClient returns an authenticated http.Client.
// On first run it starts a local server to handle the OAuth2 callback automatically.
func getHTTPClient(ctx context.Context) (*http.Client, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set")
	}

	tokPath := tokenFilePath()
	tok, err := loadToken(tokPath)
	if err != nil {
		tok, err = runLocalhostAuth(ctx, clientID, clientSecret)
		if err != nil {
			return nil, fmt.Errorf("OAuth2 auth failed: %w", err)
		}
		if err := saveToken(tokPath, tok); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save token to %s: %v\n", tokPath, err)
		}
	}

	cfg := newOAuth2Config("") // redirect_url is not needed for token exchange
	return cfg.Client(ctx, tok), nil
}

// runLocalhostAuth runs OAuth2 flow using a temporary localhost HTTP server.
func runLocalhostAuth(ctx context.Context, clientID, clientSecret string) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on localhost: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d", port)

	cfg := newOAuth2Config(redirectURL)
	cfg.ClientID = clientID
	cfg.ClientSecret = clientSecret

	state := "dev-stats-auth"
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			errCh <- fmt.Errorf("no code received: %s", errMsg)
			http.Error(w, "no code received", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "<html><body><h2>Authentication complete. You can close this window.</h2></body></html>")
		codeCh <- code
	})

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Println("Opening browser for Google authentication...")
	if err := openBrowser(authURL); err != nil {
		fmt.Println("Could not open browser automatically. Please open the following URL manually:")
		fmt.Println(authURL)
	}

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		srv.Close()
		return nil, err
	case <-ctx.Done():
		srv.Close()
		return nil, ctx.Err()
	}

	srv.Close()

	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange auth code: %w", err)
	}

	return tok, nil
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// newDriveService returns an authenticated Google Drive API service.
func newDriveService(ctx context.Context, client *http.Client) (*drive.Service, error) {
	return drive.NewService(ctx, option.WithHTTPClient(client))
}

// myUserInfo holds the authenticated user's identity.
type myUserInfo struct {
	DisplayName  string
	EmailAddress string
}

// getMyUserInfo returns the authenticated user's display name and email via Drive about API.
func getMyUserInfo(svc *drive.Service) (myUserInfo, error) {
	about, err := svc.About.Get().Fields("user").Do()
	if err != nil {
		return myUserInfo{}, fmt.Errorf("failed to get Drive user info: %w", err)
	}
	if about.User == nil {
		return myUserInfo{}, fmt.Errorf("Drive about API returned no user")
	}
	return myUserInfo{
		DisplayName:  about.User.DisplayName,
		EmailAddress: about.User.EmailAddress,
	}, nil
}
