package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type mavenResponse struct {
	Response struct {
		NumFound int `json:"numFound"`
	} `json:"response"`
}

type mavenResult struct {
	Date         string `json:"date"`
	NewArtifacts int    `json:"new_artifacts"`
}

func MavenNewArtifacts(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24*time.Hour - time.Millisecond)

	query := fmt.Sprintf("timestamp:[%d TO %d]", startOfDay.UnixMilli(), endOfDay.UnixMilli())

	u := "https://search.maven.org/solrsearch/select?" + url.Values{
		"q":    {query},
		"core": {"ga"},
		"rows": {"0"},
		"wt":   {"json"},
	}.Encode()

	resp, err := httpClient.Get(u)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query maven central: %v", err), http.StatusBadGateway)
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("maven central returned %d: %s", resp.StatusCode, body), http.StatusBadGateway)
		return
	}

	var maven mavenResponse
	if err := json.NewDecoder(resp.Body).Decode(&maven); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mavenResult{
		Date:         now.Format("2006-01-02"),
		NewArtifacts: maven.Response.NumFound,
	})
}
