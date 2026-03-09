package mcp

import "fmt"

// RegisterDefaultPrompts registers all Jenkins prompt templates with the server.
func RegisterDefaultPrompts(s *Server) {
	s.AddPrompt(
		Prompt{
			Name:        "diagnose_build_failure",
			Description: "Guide for diagnosing a failed Jenkins build",
			Arguments: []PromptArgument{
				{Name: "job", Description: "Job name or path", Required: true},
				{Name: "number", Description: "Build number (defaults to last build)", Required: false},
			},
		},
		handleDiagnoseBuildFailure,
	)

	s.AddPrompt(
		Prompt{
			Name:        "review_job_config",
			Description: "Guide for reviewing a Jenkins job configuration for best practices",
			Arguments: []PromptArgument{
				{Name: "job", Description: "Job name or path", Required: true},
			},
		},
		handleReviewJobConfig,
	)

	s.AddPrompt(
		Prompt{
			Name:        "summarize_build_history",
			Description: "Guide for analyzing build history trends of a job",
			Arguments: []PromptArgument{
				{Name: "job", Description: "Job name or path", Required: true},
				{Name: "limit", Description: "Number of recent builds to analyze (default: 10)", Required: false},
			},
		},
		handleSummarizeBuildHistory,
	)

	s.AddPrompt(
		Prompt{
			Name:        "validate_jenkinsfile",
			Description: "Guide for validating and improving a Jenkinsfile",
			Arguments: []PromptArgument{
				{Name: "jenkinsfile", Description: "Jenkinsfile content to validate", Required: true},
			},
		},
		handleValidateJenkinsfile,
	)
}

func handleDiagnoseBuildFailure(args map[string]string) (*PromptGetResult, error) {
	job := args["job"]
	if job == "" {
		return nil, fmt.Errorf("missing required argument: job")
	}

	number := args["number"]
	buildRef := "the last build"
	if number != "" {
		buildRef = fmt.Sprintf("build #%s", number)
	}

	text := fmt.Sprintf(`Diagnose the failure of %s for Jenkins job '%s'. Follow these steps:

1. First, use the build_view tool (or build_last if no number specified) to get the build details including status, duration, and timestamp.
2. Use the build_log tool to retrieve the console output and identify the error.
3. If it's a pipeline job, use pipeline_stages to see which stage failed, then pipeline_stage_log to get the specific stage output.
4. Check if the failure is:
   - A compilation/build error (check the error messages in the log)
   - A test failure (look for test result summaries)
   - An infrastructure issue (timeout, agent offline, out of disk space)
   - A configuration problem (missing credentials, wrong parameters)
5. Look at the build's changeset to see what code changes might have caused the failure.
6. Compare with previous successful builds if needed using build_list.
7. Provide a summary of:
   - What failed and why
   - The root cause
   - Suggested fix`, buildRef, job)

	return &PromptGetResult{
		Description: fmt.Sprintf("Diagnose build failure for %s", job),
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: NewTextContent(text),
			},
		},
	}, nil
}

func handleReviewJobConfig(args map[string]string) (*PromptGetResult, error) {
	job := args["job"]
	if job == "" {
		return nil, fmt.Errorf("missing required argument: job")
	}

	text := fmt.Sprintf(`Review the configuration of Jenkins job '%s'. Follow these steps:

1. Use job_view to get the current job details (health, last build status, etc).
2. Read the job's config.xml via the jenkins:///%s/config.xml resource.
3. Analyze the configuration for:
   - Build triggers: Are they appropriate? (SCM polling, webhooks, cron)
   - Build parameters: Are they well-documented with sensible defaults?
   - Source code management: Is the branch specifier correct?
   - Build steps: Are they efficient and well-ordered?
   - Post-build actions: Are notifications and artifact archiving configured?
   - Pipeline syntax: If Jenkinsfile-based, is the pipeline well-structured?
4. Check for common issues:
   - Missing or hardcoded credentials (should use credential bindings)
   - No timeout configured (builds could hang forever)
   - No retry strategy for flaky steps
   - Workspace cleanup not configured
   - Missing error handling in pipeline scripts
5. Provide recommendations for improvements.`, job, job)

	return &PromptGetResult{
		Description: fmt.Sprintf("Review job configuration for %s", job),
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: NewTextContent(text),
			},
		},
	}, nil
}

func handleSummarizeBuildHistory(args map[string]string) (*PromptGetResult, error) {
	job := args["job"]
	if job == "" {
		return nil, fmt.Errorf("missing required argument: job")
	}

	limit := args["limit"]
	if limit == "" {
		limit = "10"
	}

	text := fmt.Sprintf(`Analyze the build history of Jenkins job '%s'. Follow these steps:

1. Use build_list with limit=%s to get recent builds.
2. For each build, note the status (SUCCESS, FAILURE, UNSTABLE, ABORTED), duration, and timestamp.
3. Analyze patterns:
   - Success rate: What percentage of builds succeed?
   - Failure patterns: Are failures clustered or sporadic?
   - Duration trends: Are builds getting slower over time?
   - Stability: How often does the build alternate between success and failure?
4. If there are failures, use build_view on a few failed builds to understand common failure modes.
5. Provide a summary including:
   - Overall health score
   - Success/failure rate
   - Average build duration
   - Any concerning trends
   - Recommendations for improving stability`, job, limit)

	return &PromptGetResult{
		Description: fmt.Sprintf("Summarize build history for %s", job),
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: NewTextContent(text),
			},
		},
	}, nil
}

func handleValidateJenkinsfile(args map[string]string) (*PromptGetResult, error) {
	jenkinsfile := args["jenkinsfile"]
	if jenkinsfile == "" {
		return nil, fmt.Errorf("missing required argument: jenkinsfile")
	}

	text := fmt.Sprintf(`Validate and review the following Jenkinsfile:

%s

Follow these steps:

1. Use the pipeline_validate tool to check the syntax against the Jenkins server.
2. If validation fails, identify and explain the syntax errors.
3. Review the pipeline for best practices:
   - Are stages well-defined and logically organized?
   - Is error handling present (try/catch/finally)?
   - Are credentials accessed securely (withCredentials)?
   - Is there proper cleanup in post sections?
   - Are timeouts set to prevent hanging builds?
   - Are agents specified appropriately?
   - Is parallelism used where beneficial?
4. Check for common anti-patterns:
   - Shell commands that could fail silently
   - Missing 'script' blocks in declarative pipelines
   - Hardcoded values that should be parameters
   - Missing input validation for parameters
5. Provide the validated/improved Jenkinsfile if changes are needed.`, jenkinsfile)

	return &PromptGetResult{
		Description: "Validate and review Jenkinsfile",
		Messages: []PromptMessage{
			{
				Role:    "user",
				Content: NewTextContent(text),
			},
		},
	}, nil
}
