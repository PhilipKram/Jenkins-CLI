package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/PhilipKram/jenkins-cli/internal/errors"
)

type Job struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Color       string `json:"color"`
	Class       string `json:"_class"`
	Description string `json:"description"`
	Buildable   bool   `json:"buildable"`
	FullName    string `json:"fullName"`
}

type JobDetail struct {
	Job
	LastBuild       *BuildRef `json:"lastBuild"`
	LastSuccessBuild *BuildRef `json:"lastSuccessfulBuild"`
	LastFailedBuild  *BuildRef `json:"lastFailedBuild"`
	HealthReport     []Health  `json:"healthReport"`
	InQueue          bool      `json:"inQueue"`
	NextBuildNumber  int       `json:"nextBuildNumber"`
}

type BuildRef struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type Health struct {
	Description string `json:"description"`
	Score       int    `json:"score"`
}

type jobListResponse struct {
	Jobs []Job `json:"jobs"`
}

func (c *Client) ListJobs(ctx context.Context, folder string) ([]Job, error) {
	path := "/api/json?tree=jobs[name,url,color,_class,fullName,description,buildable]"
	if folder != "" {
		path = "/job/" + encodeJobPath(folder) + "/api/json?tree=jobs[name,url,color,_class,fullName,description,buildable]"
	}

	var resp jobListResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	return resp.Jobs, nil
}

func (c *Client) GetJob(ctx context.Context, name string) (*JobDetail, error) {
	path := "/job/" + encodeJobPath(name) + "/api/json"
	var job JobDetail
	if err := c.get(ctx, path, &job); err != nil {
		return nil, c.wrapNotFoundError(err, name)
	}
	return &job, nil
}

func (c *Client) BuildJob(ctx context.Context, name string, params map[string]string) error {
	var path string
	if len(params) > 0 {
		path = "/job/" + encodeJobPath(name) + "/buildWithParameters"
		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}
		return c.wrapNotFoundError(c.postWithCrumb(ctx, path+"?"+values.Encode(), nil), name)
	}
	path = "/job/" + encodeJobPath(name) + "/build"
	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), name)
}

func (c *Client) DisableJob(ctx context.Context, name string) error {
	path := "/job/" + encodeJobPath(name) + "/disable"
	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), name)
}

func (c *Client) EnableJob(ctx context.Context, name string) error {
	path := "/job/" + encodeJobPath(name) + "/enable"
	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), name)
}

func (c *Client) DeleteJob(ctx context.Context, name string) error {
	path := "/job/" + encodeJobPath(name) + "/doDelete"
	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), name)
}

func encodeJobPath(name string) string {
	parts := strings.Split(name, "/")
	encoded := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		encoded = append(encoded, url.PathEscape(p))
		if i < len(parts)-1 {
			encoded = append(encoded, "job")
		}
	}
	return strings.Join(encoded, "/")
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
// This is used for fuzzy matching job names.
func levenshteinDistance(s1, s2 string) int {
	s1Lower := strings.ToLower(s1)
	s2Lower := strings.ToLower(s2)

	if len(s1Lower) == 0 {
		return len(s2Lower)
	}
	if len(s2Lower) == 0 {
		return len(s1Lower)
	}

	// Create a matrix to store distances
	matrix := make([][]int, len(s1Lower)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2Lower)+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2Lower); j++ {
		matrix[0][j] = j
	}

	// Calculate distances
	for i := 1; i <= len(s1Lower); i++ {
		for j := 1; j <= len(s2Lower); j++ {
			cost := 0
			if s1Lower[i-1] != s2Lower[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1Lower)][len(s2Lower)]
}

// min returns the minimum of three integers.
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// findSimilarJobNames finds job names similar to the target name.
// Returns up to maxSuggestions similar names, sorted by similarity.
func (c *Client) findSimilarJobNames(targetName string, maxSuggestions int) []string {
	// Try to get the list of jobs
	jobs, err := c.ListJobs(context.Background(), "")
	if err != nil {
		// If we can't list jobs, return empty suggestions
		return nil
	}

	type jobDistance struct {
		name     string
		distance int
	}

	var candidates []jobDistance
	maxDistance := len(targetName) / 2 // Only suggest jobs within reasonable edit distance

	for _, job := range jobs {
		distance := levenshteinDistance(targetName, job.Name)
		// Include jobs that are reasonably similar
		if distance <= maxDistance && distance > 0 {
			candidates = append(candidates, jobDistance{name: job.Name, distance: distance})
		}
	}

	// Sort by distance (most similar first)
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].distance < candidates[i].distance {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Return top suggestions
	suggestions := make([]string, 0, maxSuggestions)
	for i := 0; i < len(candidates) && i < maxSuggestions; i++ {
		suggestions = append(suggestions, candidates[i].name)
	}

	return suggestions
}

// wrapNotFoundError wraps a not found error with job name suggestions.
func (c *Client) wrapNotFoundError(err error, jobName string) error {
	if err == nil {
		return nil
	}

	// Check if it's already a NotFoundError
	if notFoundErr, ok := errors.AsNotFoundError(err); ok {
		// If it already has suggestions, return as is
		if len(notFoundErr.Suggestions) > 0 {
			return err
		}

		// Add job suggestions
		suggestions := c.findSimilarJobNames(jobName, 5)
		if len(suggestions) > 0 {
			notFoundErr.Suggestions = suggestions
		}
		return notFoundErr
	}

	// Check if it's a 404 error that should be wrapped
	if errors.IsNotFound(err) {
		suggestions := c.findSimilarJobNames(jobName, 5)
		return &errors.NotFoundError{
			ResourceType: "Job",
			ResourceName: jobName,
			Err:          err,
			Suggestions:  suggestions,
		}
	}

	return err
}

// GetJobConfig retrieves the XML configuration of a job.
func (c *Client) GetJobConfig(ctx context.Context, name string) (string, error) {
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := "/job/" + encodeJobPath(name) + "/config.xml"
	stream, err := c.getStream(ctx, path)
	if err != nil {
		return "", c.wrapNotFoundError(fmt.Errorf("getting job config: %w", err), name)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("reading job config: %w", err)
	}
	return string(data), nil
}

// UpdateJobConfig updates a job's XML configuration.
func (c *Client) UpdateJobConfig(ctx context.Context, name string, configXML string) error {
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := "/job/" + encodeJobPath(name) + "/config.xml"

	crumb, _ := c.getCrumb(ctx)

	req, err := http.NewRequestWithContext(ctx, "POST", c.buildURL(path), strings.NewReader(configXML))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.Auth.Apply(req)
	req.Header.Set("Content-Type", "application/xml")
	if crumb != nil {
		req.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	}

	resp, err := retryWithBackoff(ctx, c.MaxRetries, func() (*http.Response, error) {
		return c.HTTPClient.Do(req)
	})
	if err != nil {
		return c.wrapNotFoundError(fmt.Errorf("updating job config: %w", err), name)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return c.classifyHTTPError(resp.StatusCode, c.buildURL(path), string(bodyBytes))
	}
	return nil
}

// CreateJob creates a new job with the given XML configuration.
func (c *Client) CreateJob(ctx context.Context, name string, configXML string) error {
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := "/createItem?name=" + url.QueryEscape(name)

	crumb, _ := c.getCrumb(ctx)

	req, err := http.NewRequestWithContext(ctx, "POST", c.buildURL(path), strings.NewReader(configXML))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.Auth.Apply(req)
	req.Header.Set("Content-Type", "application/xml")
	if crumb != nil {
		req.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	}

	resp, err := retryWithBackoff(ctx, c.MaxRetries, func() (*http.Response, error) {
		return c.HTTPClient.Do(req)
	})
	if err != nil {
		return fmt.Errorf("creating job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return c.classifyHTTPError(resp.StatusCode, c.buildURL(path), string(bodyBytes))
	}
	return nil
}

func ColorToStatus(color string) string {
	if strings.HasSuffix(color, "_anime") {
		return "BUILDING"
	}
	switch {
	case strings.HasPrefix(color, "blue"):
		return "SUCCESS"
	case strings.HasPrefix(color, "red"):
		return "FAILURE"
	case strings.HasPrefix(color, "yellow"):
		return "UNSTABLE"
	case strings.HasPrefix(color, "grey"), strings.HasPrefix(color, "disabled"):
		return "DISABLED"
	case strings.HasPrefix(color, "aborted"):
		return "ABORTED"
	case strings.HasPrefix(color, "notbuilt"):
		return "NOT_BUILT"
	default:
		return color
	}
}
