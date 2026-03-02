package oauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// FlowResult holds the tokens returned from a successful OAuth flow.
type FlowResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
}

// BrowserFlow performs an OAuth2 Authorization Code flow with PKCE.
// It starts a local callback server, opens the authorization URL in the
// user's browser, and waits for the redirect with the authorization code.
func BrowserFlow(ctx context.Context, cfg *oauth2.Config) (*FlowResult, error) {
	// Find a free port for the callback server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting callback listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	cfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	verifier, err := generateVerifier()
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("generating PKCE verifier: %w", err)
	}
	challenge := challengeFromVerifier(verifier)

	state, err := generateState()
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("generating state: %w", err)
	}

	authURL := cfg.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	// Channel to receive the auth code or error
	type callbackResult struct {
		code string
		err  error
	}
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			desc := r.URL.Query().Get("error_description")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h2>Authentication failed</h2><p>%s: %s</p><p>You can close this window.</p></body></html>", errMsg, desc)
			resultCh <- callbackResult{err: fmt.Errorf("oauth error: %s — %s", errMsg, desc)}
			return
		}

		if returnedState := r.URL.Query().Get("state"); returnedState != state {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<html><body><h2>Invalid state parameter</h2><p>You can close this window.</p></body></html>")
			resultCh <- callbackResult{err: fmt.Errorf("state mismatch: possible CSRF attack")}
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<html><body><h2>Missing authorization code</h2><p>You can close this window.</p></body></html>")
			resultCh <- callbackResult{err: fmt.Errorf("no authorization code in callback")}
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Authentication successful!</h2><p>You can close this window and return to the terminal.</p></body></html>")
		resultCh <- callbackResult{code: code}
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Close()

	fmt.Println("\nOpening browser for authentication...")
	fmt.Printf("If the browser doesn't open, visit this URL:\n\n  %s\n\n", authURL)
	fmt.Println("Waiting for callback...")

	if err := openBrowser(authURL); err != nil {
		// Non-fatal: user can copy the URL manually
		fmt.Printf("Could not open browser automatically: %v\n", err)
	}

	// Wait for callback or context cancellation
	var result callbackResult
	select {
	case result = <-resultCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if result.err != nil {
		return nil, result.err
	}

	// Exchange auth code for tokens, including PKCE verifier
	token, err := cfg.Exchange(ctx, result.code,
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}

	return &FlowResult{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}, nil
}

// ManualFlow prints the authorization URL and prompts the user to paste
// the authorization code back, for headless environments.
func ManualFlow(ctx context.Context, cfg *oauth2.Config, readCode func() (string, error)) (*FlowResult, error) {
	cfg.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"

	verifier, err := generateVerifier()
	if err != nil {
		return nil, fmt.Errorf("generating PKCE verifier: %w", err)
	}
	challenge := challengeFromVerifier(verifier)

	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}

	authURL := cfg.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	fmt.Printf("\nOpen this URL in your browser to authenticate:\n\n  %s\n\n", authURL)

	code, err := readCode()
	if err != nil {
		return nil, fmt.Errorf("reading authorization code: %w", err)
	}

	token, err := cfg.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}

	return &FlowResult{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}, nil
}

// RefreshAccessToken uses a refresh token to obtain a new access token.
func RefreshAccessToken(ctx context.Context, cfg *oauth2.Config, refreshToken string) (*FlowResult, error) {
	tokenSource := cfg.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	})

	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	return &FlowResult{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}, nil
}

// BuildOAuth2Config constructs an oauth2.Config from the stored provider settings.
func BuildOAuth2Config(clientID, clientSecret, authURL, tokenURL string, scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		Scopes: scopes,
	}
}

func openBrowser(rawURL string) error {
	// Validate the URL before passing to the browser
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
	return openBrowserPlatform(u.String())
}
