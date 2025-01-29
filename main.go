package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	gitlab "github.com/xanzy/go-gitlab"
)

var (
	gitlabToken      string
	upvotesThreshold int
	mergeRequestID   int
	pullURL          string
	version          = "dev" // Default version if not set during build
	rootCmd          = &cobra.Command{
		Use:     "approve-guard",
		Short:   "Check upvotes for a GitLab MR",
		Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			checkUpvotes()
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&gitlabToken, "token", getEnv("GITLAB_TOKEN", ""), "GitLab token (or set GITLAB_TOKEN env var)")
	rootCmd.PersistentFlags().IntVar(&upvotesThreshold, "upvotes", getIntFromEnv("UPVOTES", 1), "Upvote threshold (or set UPVOTES env var)")
	rootCmd.PersistentFlags().StringVar(&pullURL, "url", getEnv("PULL_URL", ""), "Merge request URL (or set PULL_URL env var)")
	rootCmd.PersistentFlags().IntVar(&mergeRequestID, "pull-num", getIntFromEnv("PULL_NUM", 0), "Merge request ID (or set PULL_NUM env var)")

	rootCmd.Use = filepath.Base(os.Args[0])
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %s", err)
	}
}

func checkUpvotes() {
	if upvotesThreshold < 1 {
		log.Fatalf("Invalid upvotes threshold: must be ≥1 (got %d)", upvotesThreshold)
	}
	if mergeRequestID <= 0 {
		log.Fatalf("Invalid merge request ID: must be positive (got %d)", mergeRequestID)
	}
	if gitlabToken == "" {
		log.Fatalf("GitLab token is required. Use --token or set GITLAB_TOKEN environment variable.")
	}
	if pullURL == "" {
		log.Fatalf("Merge request URL is required. Use --url or set PULL_URL environment variable.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	gitlabBaseURL, err := extractProtocolAndDomain(pullURL)
	if err != nil {
		log.Fatalf("Error extracting base URL: %s", err)
	}

	client, err := gitlab.NewClient(
		gitlabToken,
		gitlab.WithBaseURL(gitlabBaseURL),
		gitlab.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	)
	if err != nil {
		log.Fatalf("Failed to create GitLab client: %s", err)
	}

	projectNamespace, err := getProjectIDFromURL(pullURL)
	if err != nil {
		log.Fatalf("Failed to extract project namespace from URL: %s", err)
	}

	projectID, err := getNumericProjectID(ctx, client, projectNamespace)
	if err != nil {
		log.Fatalf("Failed to get numeric project ID: %s", err)
	}

	log.Printf("Numeric Project ID: %d", projectID)

	mr, _, err := client.MergeRequests.GetMergeRequest(projectID, mergeRequestID, nil, gitlab.WithContext(ctx))
	if err != nil {
		log.Fatalf("Failed to get MR: %s", err)
	}

	log.Printf("Merge request Author: %s (%s)", mr.Author.Name, mr.Author.Username)

	if mr.Upvotes >= upvotesThreshold {
		log.Println("Merge request has sufficient upvotes.")
		os.Exit(0)
	} else {
		log.Println("Merge request has insufficient upvotes.")
		os.Exit(1)
	}
}

func getProjectIDFromURL(mergeRequestURL string) (string, error) {
	u, err := url.Parse(mergeRequestURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %v", err)
	}

	pathSegments := strings.Split(u.Path, "/")
	if len(pathSegments) < 5 {
		return "", fmt.Errorf("invalid URL format")
	}

	mergeRequestsIndex := -1
	for i, segment := range pathSegments {
		if segment == "-" {
			if i < len(pathSegments)-1 && pathSegments[i+1] == "merge_requests" {
				mergeRequestsIndex = i
				break
			}
		}
	}

	if mergeRequestsIndex == -1 {
		return "", fmt.Errorf("invalid URL format: missing '-/merge_requests/'")
	}

	projectPath := strings.Join(pathSegments[:mergeRequestsIndex], "/")[1:] // remove leading '/'
	return projectPath, nil
}

func extractProtocolAndDomain(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %v", err)
	}
	return fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host), nil
}

func getNumericProjectID(ctx context.Context, client *gitlab.Client, projectNamespace string) (int, error) {
	project, _, err := client.Projects.GetProject(projectNamespace, nil, gitlab.WithContext(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to fetch project: %v", err)
	}
	return project.ID, nil
}

func getIntFromEnv(envVar string, defaultValue int) int {
	valueStr := os.Getenv(envVar)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Could not convert env var '%s' to int: %s", envVar, err)
		return defaultValue
	}
	return value
}

func getEnv(envVar, defaultValue string) string {
	value := os.Getenv(envVar)
	if value == "" {
		return defaultValue
	}
	return value
}
