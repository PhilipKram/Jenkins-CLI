package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	jenkinserrors "github.com/PhilipKram/jenkins-cli/internal/errors"
)

type Build struct {
	Number    int    `json:"number"`
	URL       string `json:"url"`
	Result    string `json:"result"`
	Building  bool   `json:"building"`
	Timestamp int64  `json:"timestamp"`
	Duration  int64  `json:"duration"`
	ID        string `json:"id"`
}

type BuildDetail struct {
	Build
	DisplayName string    `json:"displayName"`
	Description string    `json:"description"`
	FullName    string    `json:"fullDisplayName"`
	Actions     []Action  `json:"actions"`
	Artifacts   []Artifact `json:"artifacts"`
	ChangeSet   ChangeSet  `json:"changeSet"`
}

type Action struct {
	Class  string  `json:"_class"`
	Causes []Cause `json:"causes,omitempty"`
}

type Cause struct {
	ShortDescription string `json:"shortDescription"`
	UserID           string `json:"userId"`
}

type Artifact struct {
	DisplayPath  string `json:"displayPath"`
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
	Size         int64  `json:"size"`
}

type ChangeSet struct {
	Items []ChangeItem `json:"items"`
	Kind  string       `json:"kind"`
}

type ChangeItem struct {
	Author  Author `json:"author"`
	Message string `json:"msg"`
	Date    string `json:"date"`
}

type Author struct {
	FullName string `json:"fullName"`
}

type buildListResponse struct {
	Builds []Build `json:"builds"`
}

type buildArtifactsResponse struct {
	Artifacts []Artifact `json:"artifacts"`
}

func (c *Client) ListBuilds(ctx context.Context, jobName string, limit int) ([]Build, error) {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/api/json?tree=builds[number,url,result,building,timestamp,duration,id]{0,%d}",
		encodeJobPath(jobName), limit)

	var resp buildListResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("listing builds for %q: %w", jobName, err)
	}
	return resp.Builds, nil
}

func (c *Client) GetBuild(ctx context.Context, jobName string, number int) (*BuildDetail, error) {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/%d/api/json", encodeJobPath(jobName), number)

	var build BuildDetail
	if err := c.get(ctx, path, &build); err != nil {
		return nil, fmt.Errorf("getting build %d of %q: %w", number, jobName, err)
	}
	return &build, nil
}

func (c *Client) GetBuildArtifacts(ctx context.Context, jobName string, number int) ([]Artifact, error) {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/%d/api/json?tree=artifacts[displayPath,fileName,relativePath,size]",
		encodeJobPath(jobName), number)

	var resp buildArtifactsResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("getting artifacts for build %d of %q: %w", number, jobName, err)
	}
	return resp.Artifacts, nil
}

func (c *Client) GetBuildLog(ctx context.Context, jobName string, number int) (io.ReadCloser, error) {
	// Use slow timeout for potentially large log download
	ctx, cancel := c.withSlowTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/%d/consoleText", encodeJobPath(jobName), number)
	return c.getStream(ctx, path)
}

// ProgressCallback is called during artifact download to report progress.
// downloaded is the number of bytes downloaded so far, total is the total size (-1 if unknown).
type ProgressCallback func(downloaded, total int64)

// DownloadArtifact downloads a build artifact to the provided writer.
// If progressCallback is not nil, it will be called periodically during download.
func (c *Client) DownloadArtifact(ctx context.Context, jobName string, buildNumber int, relativePath string, w io.Writer, progressCallback ProgressCallback) error {
	// Use slow timeout for potentially large artifact download
	ctx, cancel := c.withSlowTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/%d/artifact/%s", encodeJobPath(jobName), buildNumber, url.PathEscape(relativePath))

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("downloading artifact: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Get total size from Content-Length header
	totalSize := resp.ContentLength

	// If no progress callback, just copy directly
	if progressCallback == nil {
		_, err = io.Copy(w, resp.Body)
		return err
	}

	// Copy with progress reporting
	buf := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			progressCallback(downloaded, totalSize)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading artifact: %w", err)
		}
	}

	return nil
}

func (c *Client) StopBuild(ctx context.Context, jobName string, number int) error {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/%d/stop", encodeJobPath(jobName), number)
	return c.postWithCrumb(ctx, path, nil)
}

func (c *Client) KillBuild(ctx context.Context, jobName string, number int) error {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/%d/kill", encodeJobPath(jobName), number)
	return c.postWithCrumb(ctx, path, nil)
}

func (c *Client) DeleteBuild(ctx context.Context, jobName string, number int) error {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/%d/doDelete", encodeJobPath(jobName), number)
	return c.postWithCrumb(ctx, path, nil)
}

func (c *Client) ReplayBuild(ctx context.Context, jobName string, number int, script string) (int, error) {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	// Get the next build number before replay
	job, err := c.GetJob(ctx, jobName)
	if err != nil {
		return 0, fmt.Errorf("getting job info: %w", err)
	}
	newBuildNumber := job.NextBuildNumber

	// Prepare form data with the script
	formData := url.Values{}
	if script != "" {
		formData.Set("mainScript", script)
	}

	// POST to the replay endpoint — try /replay/run first (newer Jenkins),
	// fall back to /replay/rebuild (older Jenkins)
	encodedJob := encodeJobPath(jobName)
	formBody := formData.Encode()

	path := fmt.Sprintf("/job/%s/%d/replay/run", encodedJob, number)
	err = c.postFormWithCrumb(ctx, path, strings.NewReader(formBody))
	if err != nil && jenkinserrors.IsNotFound(err) {
		path = fmt.Sprintf("/job/%s/%d/replay/rebuild", encodedJob, number)
		err = c.postFormWithCrumb(ctx, path, strings.NewReader(formBody))
	}
	if err != nil {
		return 0, err
	}

	return newBuildNumber, nil
}

func (c *Client) GetLastBuild(ctx context.Context, jobName string) (*BuildDetail, error) {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/lastBuild/api/json", encodeJobPath(jobName))

	var build BuildDetail
	if err := c.get(ctx, path, &build); err != nil {
		return nil, fmt.Errorf("getting last build of %q: %w", jobName, err)
	}
	return &build, nil
}

func (c *Client) GetLastSuccessfulBuild(ctx context.Context, jobName string) (*BuildDetail, error) {
	// Use standard timeout for typical API operation
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := fmt.Sprintf("/job/%s/lastSuccessfulBuild/api/json", encodeJobPath(jobName))

	var build BuildDetail
	if err := c.get(ctx, path, &build); err != nil {
		return nil, fmt.Errorf("getting last successful build of %q: %w", jobName, err)
	}
	return &build, nil
}

func (b *Build) StartTime() time.Time {
	return time.UnixMilli(b.Timestamp)
}

func (b *Build) DurationStr() string {
	d := time.Duration(b.Duration) * time.Millisecond
	if b.Building {
		d = time.Since(b.StartTime())
	}
	return formatDuration(d)
}

func (b *Build) Status() string {
	if b.Building {
		return "BUILDING"
	}
	if b.Result == "" {
		return "UNKNOWN"
	}
	return b.Result
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func BuildURL(baseURL, jobName string, number int) string {
	return fmt.Sprintf("%s/job/%s/%d", baseURL, url.PathEscape(jobName), number)
}

// StreamBuildLog streams build log progressively using Jenkins' progressive text API.
// It polls the logText/progressiveText endpoint and writes new content to the writer
// as it arrives. Polling continues until the build completes (X-More-Data header is false).
func (c *Client) StreamBuildLog(ctx context.Context, jobName string, number int, writer io.Writer, pollInterval time.Duration) error {
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}

	var offset int64 = 0
	for {
		// Check for cancellation before making request
		select {
		case <-ctx.Done():
			return nil // Gracefully exit without error
		default:
		}

		path := fmt.Sprintf("/job/%s/%d/logText/progressiveText?start=%d",
			encodeJobPath(jobName), number, offset)

		resp, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			if ctx.Err() != nil {
				return nil // Graceful exit on context cancellation
			}
			return fmt.Errorf("streaming build log for %q #%d: %w", jobName, number, err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		}

		// Copy log content to writer
		_, err = io.Copy(writer, resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("writing log content: %w", err)
		}

		// Check if more data is available
		moreData := resp.Header.Get("X-More-Data")
		if moreData == "false" {
			// Build is complete
			break
		}

		// Update offset for next poll
		if textSize := resp.Header.Get("X-Text-Size"); textSize != "" {
			var newOffset int64
			if _, err := fmt.Sscanf(textSize, "%d", &newOffset); err == nil {
				offset = newOffset
			}
		}

		// Context-aware wait before next poll
		select {
		case <-ctx.Done():
			return nil // Gracefully exit without error
		case <-time.After(pollInterval):
			// Continue to next poll
		}
	}

	return nil
}
