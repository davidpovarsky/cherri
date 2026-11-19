package main

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const documentationSite = "https://davidpovarsky.github.io/cherrilang.org/"

var documentationSourceArtifacts = []string{"docs-index.md", "docs-guide.md", "docs-actions.md", "docs-migration.md"}
var documentationSkillPaths = []string{"scripts/search-docs.sh", "scripts/common.sh", "compatibility-manifest.json"}
var documentationSkillArtifacts = []string{"installed-search-docs.sh.txt", "installed-common.sh.txt", "documentation-compatibility-manifest.json"}
var documentationSearchCases = []string{
	"active-empty", "active-show", "legacy-empty", "legacy-query",
	"absent-query", "invalid-regex", "missing-current", "restored-active",
}
var documentationHTTPRoutes = []struct{ route, artifact string }{
	{"", "published-home.html"}, {"fork-language/", "published-index.html"},
	{"fork-language/guide", "published-guide.html"}, {"fork-language/actions", "published-actions.html"},
	{"fork-language/migration", "published-migration.html"},
	{"assets/js/search-data.json", "published-search-data.json"}, {"assets/preview.js", "published-preview.js"},
}

type documentationSearchCase struct {
	Name          string   `json:"name"`
	Command       []string `json:"command"`
	Cwd           string   `json:"cwd"`
	DocsDirectory string   `json:"docsDirectory"`
	ExitCode      *int     `json:"exitCode"`
	TimedOut      *bool    `json:"timedOut"`
	StdoutSHA256  string   `json:"stdoutSHA256"`
	StderrSHA256  string   `json:"stderrSHA256"`
}

type documentationHTTPObservation struct {
	Route    string            `json:"route"`
	URL      string            `json:"url"`
	Status   int               `json:"status"`
	Bytes    int               `json:"bytes"`
	SHA256   string            `json:"sha256"`
	Artifact string            `json:"artifact"`
	Headers  map[string]string `json:"headers"`
}

type documentationConsumerReport struct {
	FormatVersion      int                            `json:"formatVersion"`
	Mode               string                         `json:"mode"`
	ImplementationSHA  string                         `json:"implementationSha"`
	CompilerCommit     string                         `json:"compilerCommit"`
	DocsCommit         string                         `json:"docsCommit"`
	SchemaFingerprint  string                         `json:"schemaFingerprint"`
	CodecFingerprint   string                         `json:"codecFingerprint"`
	MacroFingerprint   string                         `json:"macroFingerprint"`
	SourceOrigin       string                         `json:"sourceOrigin"`
	DocsOrigin         string                         `json:"docsOrigin"`
	SourceClean        bool                           `json:"sourceClean"`
	DocsClean          bool                           `json:"docsClean"`
	SourceDirectory    string                         `json:"sourceDirectory"`
	InstalledSkill     string                         `json:"installedSkill"`
	DocsDirectory      string                         `json:"docsDirectory"`
	SourceFiles        []documentationSourceFile      `json:"sourceFiles"`
	SkillFiles         []documentationSourceFile      `json:"skillFiles"`
	SearchCases        []documentationSearchCase      `json:"searchCases"`
	HTTPObservations   []documentationHTTPObservation `json:"httpObservations"`
	Before             map[string]string              `json:"before"`
	After              map[string]string              `json:"after"`
	SiteBuildSHA256    string                         `json:"siteBuildSHA256"`
	SiteControlsSHA256 string                         `json:"siteControlsSHA256"`
	DeploymentSHA256   string                         `json:"deploymentSHA256"`
	ObservedAtUTC      string                         `json:"observedAtUTC"`
	ElapsedSeconds     float64                        `json:"elapsedSeconds"`
}

type documentationSiteBuild struct {
	FormatVersion               int      `json:"formatVersion"`
	BaseURL                     string   `json:"baseUrl"`
	PublicationOrigin           string   `json:"publicationOrigin"`
	CurrentSearchRoutes         []string `json:"currentSearchRoutes"`
	VerifiedLocalLinks          int      `json:"verifiedLocalLinks"`
	HistoricalSearchEntries     *int     `json:"historicalSearchEntries"`
	DocsCommit                  string   `json:"docsCommit"`
	CompilerCommit              string   `json:"compilerCommit"`
	SourceTreeClean             bool     `json:"sourceTreeClean"`
	SourceReferenceSHA256       string   `json:"sourceReferenceSha256"`
	RenderedActionPage          string   `json:"renderedActionPage"`
	RenderedActionPageSHA256    string   `json:"renderedActionPageSha256"`
	SearchIndexSHA256           string   `json:"searchIndexSha256"`
	SchemaFingerprints          []string `json:"schemaFingerprints"`
	NativeActionCount           int      `json:"nativeActionCount"`
	RenderedReferenceSignatures int      `json:"renderedReferenceSignatures"`
	SearchEntries               int      `json:"searchEntries"`
	ActionSearchEntries         int      `json:"actionSearchEntries"`
	SitePublished               *bool    `json:"sitePublished"`
}

func documentationDocsRepository(pins documentationPins) (string, error) {
	var root = os.Getenv("CHERRI_D03_DOCS_REPO")
	if root == "" {
		root = filepath.Join(resolveRepoRoot(), "..", "cherrilang.org")
	}
	var ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var command = exec.CommandContext(ctx, "git", "-C", root, "remote", "get-url", "origin")
	command.WaitDelay = 2 * time.Second
	var origin, err = command.Output()
	if err != nil || strings.TrimSpace(string(origin)) != pins.DocsRepo {
		return "", fmt.Errorf("canonical pinned docs Git objects are unavailable; configure CHERRI_D03_DOCS_REPO")
	}
	return root, nil
}

func validateDocumentationSearch(row documentationSearchCase, expectedName string, report documentationConsumerReport, artifacts map[string][]byte, identities map[string]EvidenceArtifact) error {
	if row.Name != expectedName || row.ExitCode == nil || row.TimedOut == nil || *row.TimedOut ||
		row.Cwd != report.InstalledSkill || len(row.Command) < 2 ||
		filepath.Base(row.Command[0]) != "sh" || row.Command[1] != report.InstalledSkill+"/scripts/search-docs.sh" {
		return fmt.Errorf("D03 actual installed search process metadata is absent or inconsistent")
	}
	var arguments = map[string][]string{
		"active-empty": {}, "active-show": {"show"}, "legacy-empty": {"--legacy-reference"},
		"legacy-query": {"--legacy-reference", "action"}, "absent-query": {"CHERRI_D03_NO_MATCH_6ce5e096b6a7"},
		"invalid-regex": {"["}, "missing-current": {"CHERRI_D03_HISTORICAL_SENTINEL_49CE"}, "restored-active": {"show"},
	}
	if !reflect.DeepEqual(row.Command[2:], arguments[row.Name]) {
		return fmt.Errorf("D03 actual installed search command differs from its exact scenario")
	}
	var stdout = artifacts[row.Name+".stdout.log"]
	var stderr = artifacts[row.Name+".stderr.log"]
	if identities[row.Name+".stdout.log"].SHA256 != row.StdoutSHA256 ||
		identities[row.Name+".stderr.log"].SHA256 != row.StderrSHA256 ||
		!documentationHash(row.StdoutSHA256, 64) || !documentationHash(row.StderrSHA256, 64) ||
		!utf8.Valid(stdout) || !utf8.Valid(stderr) {
		return fmt.Errorf("D03 installed search lacks exact process bytes")
	}
	if row.Name == "missing-current" {
		if row.DocsDirectory == report.DocsDirectory || !strings.HasSuffix(row.DocsDirectory, "/missing-current-docs") {
			return fmt.Errorf("D03 missing-current control is not isolated from installed docs")
		}
	} else if row.DocsDirectory != report.DocsDirectory {
		return fmt.Errorf("D03 healthy searches did not use the actual pinned docs")
	}
	var active = row.DocsDirectory + "/fork-language"
	var historical = row.DocsDirectory + "/language"
	switch row.Name {
	case "active-empty", "legacy-empty":
		var expected = active
		if row.Name == "legacy-empty" {
			expected = historical
		}
		if *row.ExitCode != 0 || len(stderr) != 0 || string(stdout) != expected+"\n" {
			return fmt.Errorf("D03 empty search did not select the exact current/explicit historical subtree")
		}
	case "active-show", "legacy-query", "restored-active":
		var prefix = active
		if row.Name == "legacy-query" {
			prefix = historical
		}
		if *row.ExitCode != 0 || len(stderr) != 0 || len(stdout) == 0 || !bytes.HasSuffix(stdout, []byte("\n")) {
			return fmt.Errorf("D03 healthy search failed or produced no actual result")
		}
		var linePattern = regexp.MustCompile("^" + regexp.QuoteMeta(prefix) + "/[^:\n]+:[1-9][0-9]*:")
		var actionFound bool
		for _, line := range strings.Split(strings.TrimSuffix(string(stdout), "\n"), "\n") {
			if !linePattern.MatchString(line) {
				return fmt.Errorf("D03 search emitted a historical or out-of-tree result")
			}
			if strings.HasPrefix(line, active+"/actions.md:") && strings.Contains(strings.ToLower(line), "show") {
				actionFound = true
			}
		}
		if row.Name != "legacy-query" && !actionFound {
			return fmt.Errorf("D03 search did not select the generated current action reference")
		}
	case "absent-query":
		if *row.ExitCode != 0 || len(stdout) != 0 || len(stderr) != 0 {
			return fmt.Errorf("D03 no-match query is not an empty successful result")
		}
	case "invalid-regex":
		if *row.ExitCode < 2 || len(stdout) != 0 || len(stderr) == 0 {
			return fmt.Errorf("D03 execution-error control was swallowed")
		}
	case "missing-current":
		if *row.ExitCode != 1 || len(stdout) != 0 ||
			string(stderr) != "Active V2 documentation directory is missing or linked: "+active+"\n" {
			return fmt.Errorf("D03 missing-current control did not fail closed with its exact diagnostic")
		}
	}
	return nil
}

func validateDocumentationSite(report documentationConsumerReport, artifacts map[string][]byte, identities map[string]EvidenceArtifact, pins documentationPins, sources map[string]string) error {
	if identities["documentation-site-build.json"].SHA256 != report.SiteBuildSHA256 ||
		identities["documentation-site-controls.json"].SHA256 != report.SiteControlsSHA256 ||
		identities["documentation-site-deployment.json"].SHA256 != report.DeploymentSHA256 {
		return fmt.Errorf("D03 site build, controls or deployment metadata are unbound")
	}
	var build documentationSiteBuild
	if err := decodeDocumentationJSON(artifacts["documentation-site-build.json"], &build); err != nil {
		return fmt.Errorf("D03 invalid site build receipt: %w", err)
	}
	var routes = []string{"/cherrilang.org", "/cherrilang.org/fork-language", "/cherrilang.org/fork-language/actions",
		"/cherrilang.org/fork-language/guide", "/cherrilang.org/fork-language/migration"}
	if build.FormatVersion != 2 || build.BaseURL != "/cherrilang.org" || build.PublicationOrigin != "https://davidpovarsky.github.io" ||
		!reflect.DeepEqual(build.CurrentSearchRoutes, routes) || build.VerifiedLocalLinks <= 0 ||
		build.HistoricalSearchEntries == nil || *build.HistoricalSearchEntries != 0 ||
		build.DocsCommit != pins.DocsCommit || build.CompilerCommit != pins.CompilerCommit || !build.SourceTreeClean ||
		build.SourceReferenceSHA256 != sources["fork-language/actions.md"] ||
		build.RenderedActionPage != "fork-language/actions.html" || build.SitePublished == nil || *build.SitePublished ||
		build.NativeActionCount <= 0 || build.RenderedReferenceSignatures <= 0 || build.SearchEntries <= 0 || build.ActionSearchEntries <= 0 {
		return fmt.Errorf("D03 build receipt is stale or does not establish the current canonical site")
	}
	var fingerprints = map[string]string{}
	for _, value := range build.SchemaFingerprints {
		fingerprints[value] = value
	}
	if len(build.SchemaFingerprints) != 2 || fingerprints[pins.SchemaFingerprint] == "" || fingerprints[pins.MacroFingerprint] == "" {
		return fmt.Errorf("D03 site schema/macro fingerprints differ")
	}
	var controls struct {
		FormatVersion            int    `json:"formatVersion"`
		DocsCommit               string `json:"docsCommit"`
		CompilerCommit           string `json:"compilerCommit"`
		Passed                   int    `json:"passed"`
		Failed                   *int   `json:"failed"`
		Skipped                  *int   `json:"skipped"`
		ArtifactsRestoredExactly bool   `json:"artifactsRestoredExactly"`
		Controls                 []struct {
			Name       string `json:"name"`
			Rejected   bool   `json:"rejected"`
			Diagnostic string `json:"diagnostic"`
		} `json:"controls"`
	}
	if err := decodeDocumentationJSON(artifacts["documentation-site-controls.json"], &controls); err != nil ||
		controls.FormatVersion != 1 || controls.DocsCommit != pins.DocsCommit || controls.CompilerCommit != pins.CompilerCommit ||
		controls.Passed != 6 || controls.Failed == nil || *controls.Failed != 0 || controls.Skipped == nil || *controls.Skipped != 0 ||
		!controls.ArtifactsRestoredExactly || len(controls.Controls) != 6 {
		return fmt.Errorf("D03 site causal control receipt is incomplete")
	}
	var requiredControls = []struct{ name, diagnostic string }{
		{"historical-route-in-default-search", "Default search includes a historical route:"},
		{"search-result-without-project-base", "Search result URL does not match"},
		{"client-search-request-without-project-base", "Search client does not load"},
		{"missing-home-authoring-entry", "Home page lacks its unique current-guide"},
		{"missing-home-image", "Built site is missing a unique page:"},
		{"damaged-canonical-action-fingerprint", "rendered action page lost canonical metadata:"},
	}
	for i, expected := range requiredControls {
		var actual = controls.Controls[i]
		if actual.Name != expected.name || !actual.Rejected || !strings.HasPrefix(actual.Diagnostic, expected.diagnostic) {
			return fmt.Errorf("D03 site causal scenario or observed diagnostic differs")
		}
	}
	var deployment, err = decodeNativeEncodingObject(artifacts["documentation-site-deployment.json"])
	if err != nil || deployment["headSha"] != pins.DocsCommit || deployment["conclusion"] != "success" || deployment["status"] != "completed" {
		return fmt.Errorf("D03 publication has no successful deployment at the pinned docs revision")
	}
	var deploymentURL, validURL = deployment["url"].(string)
	if !validURL || !regexp.MustCompile("^https://github.com/davidpovarsky/cherrilang\\.org/actions/runs/[1-9][0-9]*$").MatchString(deploymentURL) {
		return fmt.Errorf("D03 deployment is not the canonical docs repository")
	}
	var jobs, validJobs = deployment["jobs"].([]any)
	var expectedJobs = map[string]int{"build": 0, "deploy": 0}
	if !validJobs {
		return fmt.Errorf("D03 publication omits actual build/deploy outcomes")
	}
	for _, value := range jobs {
		var job, ok = value.(map[string]any)
		if !ok {
			return fmt.Errorf("D03 invalid deployment job")
		}
		var name, _ = job["name"].(string)
		if _, required := expectedJobs[name]; required {
			if job["status"] != "completed" || job["conclusion"] != "success" {
				return fmt.Errorf("D03 required deployment job failed")
			}
			expectedJobs[name]++
		}
	}
	if expectedJobs["build"] != 1 || expectedJobs["deploy"] != 1 {
		return fmt.Errorf("D03 deployment jobs are missing or duplicated")
	}
	if len(report.HTTPObservations) != len(documentationHTTPRoutes) {
		return fmt.Errorf("D03 public routes were not all observed")
	}
	for i, expected := range documentationHTTPRoutes {
		var observed = report.HTTPObservations[i]
		var raw, exists = artifacts[expected.artifact]
		if !exists || len(raw) == 0 || observed.Route != expected.route ||
			observed.URL != documentationSite+expected.route || observed.Status != 200 ||
			observed.Artifact != expected.artifact || observed.Bytes != len(raw) ||
			observed.SHA256 != identities[expected.artifact].SHA256 || len(observed.Headers) == 0 {
			return fmt.Errorf("D03 published HTTP bytes, status or exact project URL are missing")
		}
		var contentType = strings.TrimSpace(strings.SplitN(observed.Headers["Content-Type"], ";", 2)[0])
		var expectedType = "text/html"
		if strings.HasSuffix(expected.route, ".json") {
			expectedType = "application/json"
		}
		if strings.HasSuffix(expected.route, ".js") {
			expectedType = "application/javascript"
		}
		if contentType != expectedType && !(expectedType == "application/javascript" && contentType == "text/javascript") {
			return fmt.Errorf("D03 public response has the wrong observed content type")
		}
	}
	if identities["published-actions.html"].SHA256 != build.RenderedActionPageSHA256 ||
		identities["published-search-data.json"].SHA256 != build.SearchIndexSHA256 {
		return fmt.Errorf("D03 live action/search bytes differ from the pinned deployment build")
	}
	var search, searchErr = decodeNativeEncodingObject(artifacts["published-search-data.json"])
	if searchErr != nil || len(search) != build.SearchEntries {
		return fmt.Errorf("D03 published default search count differs from its build")
	}
	var allowed = map[string]bool{"": true, "/fork-language": true, "/fork-language/actions": true,
		"/fork-language/guide": true, "/fork-language/migration": true}
	var actionRows int
	for _, value := range search {
		var row, ok = value.(map[string]any)
		if !ok {
			return fmt.Errorf("D03 published search contains a malformed row")
		}
		for _, key := range []string{"doc", "title", "content", "url", "relUrl"} {
			if _, ok := row[key].(string); !ok {
				return fmt.Errorf("D03 published search field is missing or not text")
			}
		}
		var relative = row["relUrl"].(string)
		var route = strings.TrimRight(strings.SplitN(relative, "#", 2)[0], "/")
		if !allowed[route] || strings.Contains(relative, "?") || row["url"] != "/cherrilang.org"+relative {
			return fmt.Errorf("D03 published default search includes historical or external paths")
		}
		if route == "/fork-language/actions" {
			actionRows++
		}
	}
	if actionRows != build.ActionSearchEntries {
		return fmt.Errorf("D03 published generated-action search count differs")
	}
	return nil
}

func validateDocumentationConsumers(record EvidenceRecord, artifacts map[string][]byte, identities map[string]EvidenceArtifact, pins documentationPins) error {
	if len(artifacts) != 34 {
		return fmt.Errorf("D03 docs proof requires the complete 34 real consumer artifacts")
	}
	var report documentationConsumerReport
	if err := decodeDocumentationJSON(artifacts["documentation-consumers.json"], &report); err != nil {
		return fmt.Errorf("D03 consumer report is invalid: %w", err)
	}
	if report.FormatVersion != 1 || report.Mode != "actual-installed-public-site" ||
		report.ImplementationSHA != record.ImplementationSHA || report.CompilerCommit != pins.CompilerCommit ||
		report.DocsCommit != pins.DocsCommit || report.SchemaFingerprint != pins.SchemaFingerprint ||
		report.CodecFingerprint != pins.CodecFingerprint || report.MacroFingerprint != pins.MacroFingerprint ||
		report.SourceOrigin != pins.CompilerRepo || report.DocsOrigin != pins.DocsRepo ||
		!report.SourceClean || !report.DocsClean || !strings.HasPrefix(report.SourceDirectory, "/") ||
		!strings.HasPrefix(report.InstalledSkill, "/") || !strings.HasPrefix(report.DocsDirectory, "/") ||
		report.InstalledSkill == report.SourceDirectory+"/skills/cherri-shortcuts" ||
		report.ElapsedSeconds <= 0 || math.IsNaN(report.ElapsedSeconds) || math.IsInf(report.ElapsedSeconds, 0) {
		return fmt.Errorf("D03 installed consumer tuple or actual host execution scope differs")
	}
	if _, err := time.Parse(time.RFC3339Nano, report.ObservedAtUTC); err != nil {
		return fmt.Errorf("D03 actual observation timestamp is missing")
	}
	if len(record.Command) != 18 || record.Command[0] != "python3" ||
		record.Command[1] != "tools/p10r15/verify_documentation_consumers.py" {
		return fmt.Errorf("D03 docs proof lacks its actual collector invocation")
	}
	var flags = map[string]string{}
	for i := 2; i < len(record.Command); i += 2 {
		if flags[record.Command[i]] != "" || record.Command[i+1] == "" {
			return fmt.Errorf("D03 collector command is duplicated or incomplete")
		}
		flags[record.Command[i]] = record.Command[i+1]
	}
	if len(flags) != 8 || flags["--source"] != report.SourceDirectory || flags["--skill"] != report.InstalledSkill ||
		flags["--docs"] != report.DocsDirectory || flags["--implementation-sha"] != record.ImplementationSHA ||
		flags["--site-build"] == "" || flags["--site-controls"] == "" || flags["--deployment"] == "" || flags["--output"] == "" {
		return fmt.Errorf("D03 collector command does not bind its actual source/installed inputs")
	}
	var sources, err = documentationSourceMap(report.SourceFiles)
	if err != nil {
		return err
	}
	var docsRoot, rootErr = documentationDocsRepository(pins)
	if rootErr != nil {
		return rootErr
	}
	var before = make(map[string]string)
	for i, name := range documentationSourcePaths {
		var original, objectErr = documentationGitBlob(docsRoot, pins.DocsCommit, name)
		if objectErr != nil || !bytes.Equal(artifacts[documentationSourceArtifacts[i]], original) ||
			identities[documentationSourceArtifacts[i]].SHA256 != sources[name] {
			return fmt.Errorf("D03 installed docs source differs from the pinned canonical Git blob: %s", name)
		}
		before["docs/"+name] = sources[name]
	}
	if len(report.SkillFiles) != 3 {
		return fmt.Errorf("D03 installed wrapper/common/manifest source identities are missing")
	}
	for i, name := range documentationSkillPaths {
		var source = report.SkillFiles[i]
		var original, objectErr = documentationGitBlob(resolveRepoRoot(), record.ImplementationSHA, "skills/cherri-shortcuts/"+name)
		if objectErr != nil || source.Path != name || !bytes.Equal(artifacts[documentationSkillArtifacts[i]], original) ||
			source.SHA256 != identities[documentationSkillArtifacts[i]].SHA256 {
			return fmt.Errorf("D03 installed Skill differs from the actual immutable source object: %s", name)
		}
		before["skill/"+name] = source.SHA256
	}
	if !reflect.DeepEqual(before, report.Before) || !reflect.DeepEqual(before, report.After) {
		return fmt.Errorf("D03 installed source bytes were changed or not fully observed")
	}
	if len(report.SearchCases) != len(documentationSearchCases) {
		return fmt.Errorf("D03 installed query/control matrix is incomplete")
	}
	for i, name := range documentationSearchCases {
		if err := validateDocumentationSearch(report.SearchCases[i], name, report, artifacts, identities); err != nil {
			return err
		}
	}
	if !bytes.Equal(artifacts["active-show.stdout.log"], artifacts["restored-active.stdout.log"]) {
		return fmt.Errorf("D03 current search changed after the isolated failure control")
	}
	return validateDocumentationSite(report, artifacts, identities, pins, sources)
}
