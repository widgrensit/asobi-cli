package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Game is a single game returned by ListGames.
type Game struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type listGamesResponse struct {
	Games []Game `json:"games"`
	Error string `json:"error,omitempty"`
}

// ListGames returns the games belonging to the caller's tenant.
func ListGames(creds *Credentials) ([]Game, error) {
	req, err := http.NewRequest("GET", creds.SaasURL+"/internal/cli/games", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET /internal/cli/games: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		refreshed, refreshErr := RefreshAccessToken(creds)
		if refreshErr != nil {
			return nil, fmt.Errorf("access token expired and refresh failed: %w", refreshErr)
		}
		creds.AccessToken = refreshed
		_ = SaveCredentials(creds)
		return ListGames(creds)
	}

	var result listGamesResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list games failed (%d): %s", resp.StatusCode, result.Error)
	}
	return result.Games, nil
}

// gameQuery renders the ?game=<slug> query string used by env path operations.
func gameQuery(game string) string {
	return "?game=" + url.QueryEscape(game)
}
