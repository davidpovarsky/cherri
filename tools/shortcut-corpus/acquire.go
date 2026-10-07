/*
 * Copyright (c) Cherri
 */

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/electrikmilk/cherri/internal/icloudshortcut"
)

const acquisitionStateVersion = 2

type sourcePolicy string

const (
	policyOfficialAPI sourcePolicy = "OFFICIAL_API"
	policyPublicFeed  sourcePolicy = "PUBLIC_MACHINE_FEED"
	policyBootstrap   sourcePolicy = "BOOTSTRAP_DATASET"
	policyManualOnly  sourcePolicy = "MANUAL_ONLY"
	policyDisabled    sourcePolicy = "DISABLED"
)

type sourceInfo struct {
	Name             string
	Policy           sourcePolicy
	EnabledByDefault bool
	Reference        string
	Reason           string
}

var sourceRegistry = []sourceInfo{
	{Name: "seed", Policy: policyPublicFeed, EnabledByDefault: true, Reason: "User-supplied public iCloud share links; deterministic fallback."},
	{Name: "routinehub", Policy: policyPublicFeed, EnabledByDefault: true, Reference: "https://routinehub.co/docs/", Reason: "Best-effort public discovery adapter; isolated because the official API has no public list-all shortcut endpoint."},
	{Name: "shortcutsbench", Policy: policyBootstrap, EnabledByDefault: false, Reference: "https://github.com/EachSheep/ShortcutsBench", Reason: "Apache-2.0 historical bootstrap; minimal iCloud/provenance fields only."},
	{Name: "github-public", Policy: policyDisabled, EnabledByDefault: false, Reference: "https://docs.github.com/en/rest/search/search", Reason: "Future long-tail source; disabled until a deliberately public-only authenticated search adapter is configured."},
	{Name: "macstories", Policy: policyManualOnly, Reference: "https://www.macstories.net/terms-of-service/", Reason: "Shortcut archive is useful, but general site terms restrict automated content access/data mining."},
	{Name: "matthew-free", Policy: policyManualOnly, Reference: "https://matthewcassinelli.com/sirishortcuts/library/free/", Reason: "Free library is useful; no default automation permission is assumed."},
	{Name: "shareshortcuts", Policy: policyManualOnly, Reference: "https://shareshortcuts.com/terms.php", Reason: "Current terms prohibit automated agents/scripts generating automated requests/searches."},
	{Name: "shortcutsgallery", Policy: policyManualOnly, Reference: "https://shortcutsgallery.com/terms-of-use/", Reason: "Restrictive site-use terms; use independently supplied iCloud links instead."},
}

type discoveredShortcut struct {
	Source        string `json:"source"`
	SourceItemID  string `json:"sourceItemId,omitempty"`
	SourceItemURL string `json:"sourceItemUrl,omitempty"`
	Version       string `json:"version,omitempty"`
	ICloudURL     string `json:"icloudUrl"`
}

type acquisitionItem struct {
	Source        string `json:"source"`
	SourceItemID  string `json:"sourceItemId,omitempty"`
	SourceItemURL string `json:"sourceItemUrl,omitempty"`
	Version       string `json:"version,omitempty"`
	ICloudID      string `json:"icloudId,omitempty"`
	ICloudURL     string `json:"icloudUrl,omitempty"`
	ContentSHA256 string `json:"contentSha256,omitempty"`
	LastSeen      string `json:"lastSeen,omitempty"`
	LastFetched   string `json:"lastFetched,omitempty"`
	Status        string `json:"status,omitempty"`
	Error         string `json:"error,omitempty"`
}

type acquisitionContent struct {
	SHA256    string   `json:"sha256"`
	ByteSize  int      `json:"byteSize"`
	FirstSeen string   `json:"firstSeen"`
	Sources   []string `json:"sources,omitempty"`
	ICloudIDs []string `json:"icloudIds,omitempty"`
	Path      string   `json:"path,omitempty"`
}

type acquisitionState struct {
	Version int                            `json:"version"`
	Items   map[string]*acquisitionItem    `json:"items"`
	ICloud  map[string]string              `json:"icloud"`
	Content map[string]*acquisitionContent `json:"content"`
	Sources map[string]string              `json:"sources,omitempty"`
}

type collectConfig struct {
	Sources           string
	SeedPath          string
	Inbox             string
	StatePath         string
	MaxItems          int
	DryRun            bool
	RoutineHubFeed    string
	ShortcutsBenchURL string
	Resolver          *icloudshortcut.Resolver
}

type collectStats struct {
	Discovered       int `json:"discovered"`
	KnownItems       int `json:"knownItems"`
	Downloaded       int `json:"downloaded"`
	DuplicateContent int `json:"duplicateContent"`
	Failed           int `json:"failed"`
	DistinctContent  int `json:"distinctContent"`
}

func newAcquisitionState() *acquisitionState {
	return &acquisitionState{Version: acquisitionStateVersion, Items: map[string]*acquisitionItem{}, ICloud: map[string]string{}, Content: map[string]*acquisitionContent{}, Sources: map[string]string{}}
}

func loadAcquisitionState(path string) (*acquisitionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) { return newAcquisitionState(), nil }
		return nil, err
	}
	var state acquisitionState
	if err = json.Unmarshal(data, &state); err != nil { return nil, fmt.Errorf("corrupt acquisition state: %w", err) }
	if state.Version > acquisitionStateVersion { return nil, fmt.Errorf("unsupported acquisition state version %d", state.Version) }
	if state.Items == nil { state.Items = map[string]*acquisitionItem{} }
	if state.ICloud == nil { state.ICloud = map[string]string{} }
	if state.Content == nil { state.Content = map[string]*acquisitionContent{} }
	if state.Sources == nil { state.Sources = map[string]string{} }
	state.Version = acquisitionStateVersion
	return &state, nil
}

func (state *acquisitionState) save(path string) error {
	dir := filepath.Dir(path)
	if dir != "." { if err := os.MkdirAll(dir, 0755); err != nil { return err } }
	encoded, err := json.MarshalIndent(state, "", "  "); if err != nil { return err }
	temp := path + ".tmp"
	if err = os.WriteFile(temp, append(encoded, '\n'), 0600); err != nil { return err }
	return os.Rename(temp, path)
}

func runSources(argv []string) {
	flags := flag.NewFlagSet("sources", flag.ExitOnError); flags.Parse(argv)
	if flags.NArg() != 0 { fmt.Fprint(os.Stderr, usage); os.Exit(2) }
	fmt.Printf("%-18s %-20s %-8s %s\n", "SOURCE", "MODE", "DEFAULT", "REASON")
	for _, source := range sourceRegistry { fmt.Printf("%-18s %-20s %-8t %s\n", source.Name, source.Policy, source.EnabledByDefault, source.Reason) }
}

func addCollectFlags(flags *flag.FlagSet, config *collectConfig) {
	flags.StringVar(&config.Sources, "sources", "routinehub", "comma-separated acquisition sources")
	flags.StringVar(&config.SeedPath, "seed", "", "seed file containing public iCloud links")
	flags.StringVar(&config.Inbox, "inbox", "./corpus-inbox", "ignored raw corpus inbox")
	flags.StringVar(&config.StatePath, "acquisition-state", "./corpus-acquisition-state.json", "local acquisition state")
	flags.IntVar(&config.MaxItems, "max-items", 100, "maximum discovered items to resolve (0 = unlimited)")
	flags.BoolVar(&config.DryRun, "dry-run", false, "discover only; do not resolve/download")
	flags.StringVar(&config.RoutineHubFeed, "routinehub-feed", envOr("ROUTINEHUB_FEED_URL", "https://routinehub.co/shortcuts/latest/feed/"), "RoutineHub public discovery surface; must remain on routinehub.co")
	flags.StringVar(&config.ShortcutsBenchURL, "shortcutsbench-url", envOr("SHORTCUTSBENCH_DATA_URL", "https://raw.githubusercontent.com/EachSheep/ShortcutsBench/master/deves_dataset/dataset_src/routinehub.co.json"), "ShortcutsBench JSON dataset URL; must remain on raw.githubusercontent.com")
}

func runCollect(argv []string) {
	var config collectConfig
	flags := flag.NewFlagSet("collect", flag.ExitOnError); addCollectFlags(flags, &config); flags.Parse(argv)
	if flags.NArg() != 0 { fmt.Fprint(os.Stderr, usage); os.Exit(2) }
	stats, err := collect(config)
	if err != nil { fmt.Fprintf(os.Stderr, "Error: %v\n", err); os.Exit(1) }
	fmt.Printf("Collected: discovered=%d known=%d downloaded=%d duplicate-content=%d distinct-content=%d failed=%d\n", stats.Discovered, stats.KnownItems, stats.Downloaded, stats.DuplicateContent, stats.DistinctContent, stats.Failed)
}

func collect(config collectConfig) (collectStats, error) {
	state, err := loadAcquisitionState(config.StatePath); if err != nil { return collectStats{}, err }
	ctx := context.Background()
	var discovered []discoveredShortcut
	var sourceErrors []string
	for _, source := range splitSources(config.Sources) {
		var items []discoveredShortcut
		var discoverErr error
		switch source {
		case "seed": items, discoverErr = discoverSeed(config.SeedPath)
		case "routinehub": items, discoverErr = discoverRoutineHub(ctx, config.RoutineHubFeed)
		case "shortcutsbench": items, discoverErr = discoverShortcutsBench(ctx, config.ShortcutsBenchURL)
		default: discoverErr = fmt.Errorf("source %q is not enabled for automated collection; use a seed file with public iCloud links", source)
		}
		if discoverErr != nil {
			state.Sources[source] = "error: " + discoverErr.Error()
			sourceErrors = append(sourceErrors, source+": "+discoverErr.Error())
			continue
		}
		state.Sources[source] = "ok " + now()
		discovered = append(discovered, items...)
	}
	// Repeated iCloud IDs from different sources are provenance. Keep them here;
	// state.ICloud/content SHA dedupe prevents repeated downloads/evidence later.
	sort.SliceStable(discovered, func(i, j int) bool {
		if discovered[i].Source != discovered[j].Source { return discovered[i].Source < discovered[j].Source }
		if discovered[i].SourceItemID != discovered[j].SourceItemID { return discovered[i].SourceItemID < discovered[j].SourceItemID }
		return discovered[i].ICloudURL < discovered[j].ICloudURL
	})
	stats := collectStats{Discovered: len(discovered)}
	resolver := config.Resolver
	if resolver == nil { resolver = icloudshortcut.NewResolver() }
	processed := 0
	for _, item := range discovered {
		canonical, id, canonicalErr := icloudshortcut.CanonicalURL(item.ICloudURL)
		if canonicalErr != nil { stats.Failed++; continue }
		item.ICloudURL = canonical
		key := acquisitionItemKey(item, id)
		if existing := state.Items[key]; existing != nil && existing.ICloudID == id && existing.Version == item.Version && existing.ContentSHA256 != "" {
			existing.LastSeen = now(); mergeProvenance(state, existing.ContentSHA256, item.Source, id); stats.KnownItems++; continue
		}
		if contentSHA := state.ICloud[id]; contentSHA != "" {
			state.Items[key] = observedAcquisitionItem(item, id, canonical, contentSHA, "known", "")
			mergeProvenance(state, contentSHA, item.Source, id); stats.KnownItems++; continue
		}
		if config.MaxItems > 0 && processed >= config.MaxItems { continue }
		processed++
		if config.DryRun { continue }
		result, resolveErr := resolver.Resolve(ctx, canonical)
		if resolveErr != nil {
			state.Items[key] = observedAcquisitionItem(item, id, canonical, "", "failed", resolveErr.Error()); stats.Failed++; continue
		}
		if _, parseErr := parseDocument(id+".plist", result.Data); parseErr != nil {
			state.Items[key] = observedAcquisitionItem(item, id, canonical, "", "invalid", parseErr.Error()); stats.Failed++; continue
		}
		digest := sha256Hex(result.Data); state.ICloud[id] = digest
		content := state.Content[digest]
		if content == nil {
			dir := filepath.Join(config.Inbox, safeSourceDir(item.Source)); if err = os.MkdirAll(dir, 0700); err != nil { return stats, err }
			path := filepath.Join(dir, digest+".plist"); if err = os.WriteFile(path, result.Data, 0600); err != nil { return stats, err }
			content = &acquisitionContent{SHA256: digest, ByteSize: len(result.Data), FirstSeen: now(), Path: path}; state.Content[digest] = content; stats.DistinctContent++
		} else { stats.DuplicateContent++ }
		mergeProvenance(state, digest, item.Source, id)
		stored := observedAcquisitionItem(item, id, canonical, digest, "ok", ""); stored.LastFetched = now(); state.Items[key] = stored; stats.Downloaded++
	}
	if err = state.save(config.StatePath); err != nil { return stats, err }
	if len(discovered) == 0 && len(sourceErrors) != 0 { return stats, errors.New(strings.Join(sourceErrors, "; ")) }
	return stats, nil
}

func acquisitionItemKey(item discoveredShortcut, icloudID string) string {
	id := item.SourceItemID
	if id == "" { id = icloudID }
	return item.Source + ":" + id + ":" + icloudID
}

func observedAcquisitionItem(item discoveredShortcut, id, canonical, contentSHA, status, errorText string) *acquisitionItem {
	return &acquisitionItem{Source: item.Source, SourceItemID: item.SourceItemID, SourceItemURL: item.SourceItemURL, Version: item.Version, ICloudID: id, ICloudURL: canonical, ContentSHA256: contentSHA, LastSeen: now(), Status: status, Error: errorText}
}

func mergeProvenance(state *acquisitionState, contentSHA, source, icloudID string) {
	content := state.Content[contentSHA]
	if content == nil { return }
	content.Sources = boundedUniqueStrings(append(content.Sources, source), 64)
	content.ICloudIDs = boundedUniqueStrings(append(content.ICloudIDs, icloudID), 64)
}

func safeSourceDir(source string) string {
	var builder strings.Builder
	for _, r := range source {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' { builder.WriteRune(r) }
	}
	if builder.Len() == 0 { return "source" }
	return builder.String()
}

func sha256Hex(data []byte) string { digest := sha256.Sum256(data); return hex.EncodeToString(digest[:]) }

var icloudLinkPattern = regexp.MustCompile(`https://(?:www\.)?icloud\.com/shortcuts/[A-Za-z0-9]{16,64}`)
var routineHubShortcutPattern = regexp.MustCompile(`https://routinehub\.co/shortcut/([0-9]+)[^\s"'<>]*`)

func discoverSeed(path string) ([]discoveredShortcut, error) {
	if path == "" { return nil, errors.New("seed source requires -seed <file>") }
	data, err := os.ReadFile(path); if err != nil { return nil, err }
	return linksToDiscovered("seed", string(data)), nil
}

func discoverRoutineHub(ctx context.Context, feed string) ([]discoveredShortcut, error) {
	body, err := fetchAllowedSource(ctx, feed, 4<<20, []string{"routinehub.co", "www.routinehub.co"}); if err != nil { return nil, err }
	items := linksToDiscovered("routinehub", string(body))
	if len(items) != 0 { return items, nil }
	ids := routineHubShortcutPattern.FindAllStringSubmatch(string(body), -1)
	seen := map[string]bool{}
	for _, match := range ids {
		id := match[1]; if seen[id] { continue }; seen[id] = true
		pageURL := "https://routinehub.co/shortcut/" + id + "/"
		page, pageErr := fetchAllowedSource(ctx, pageURL, 2<<20, []string{"routinehub.co", "www.routinehub.co"}); if pageErr != nil { continue }
		for _, found := range linksToDiscovered("routinehub", string(page)) { found.SourceItemID = id; found.SourceItemURL = pageURL; items = append(items, found) }
		if len(items) >= 100 { break }
	}
	if len(items) == 0 { return nil, errors.New("RoutineHub public discovery surface yielded no iCloud links; no access controls were bypassed") }
	return items, nil
}

func discoverShortcutsBench(ctx context.Context, rawURL string) ([]discoveredShortcut, error) {
	body, err := fetchAllowedSource(ctx, rawURL, 64<<20, []string{"raw.githubusercontent.com"}); if err != nil { return nil, err }
	var records []map[string]any
	if err = json.Unmarshal(body, &records); err != nil { return nil, fmt.Errorf("ShortcutsBench dataset JSON: %w", err) }
	var items []discoveredShortcut
	for index, record := range records {
		raw, _ := record["URL"].(string); link := icloudLinkPattern.FindString(raw); if link == "" { continue }
		sourceURL, _ := record["Source"].(string)
		items = append(items, discoveredShortcut{Source: "shortcutsbench", SourceItemID: fmt.Sprintf("%d", index), SourceItemURL: sourceURL, ICloudURL: link})
	}
	return items, nil
}

func linksToDiscovered(source, text string) []discoveredShortcut {
	links := icloudLinkPattern.FindAllString(text, -1); var items []discoveredShortcut
	for index, link := range links { items = append(items, discoveredShortcut{Source: source, SourceItemID: fmt.Sprintf("%d", index), ICloudURL: link}) }
	return items
}

func splitSources(raw string) []string {
	parts := strings.Split(raw, ","); var result []string
	for _, part := range parts { part = strings.TrimSpace(part); if part != "" { result = append(result, part) } }
	return result
}

func envOr(key, fallback string) string { if value := os.Getenv(key); value != "" { return value }; return fallback }

func fetchAllowedSource(ctx context.Context, raw string, maxBytes int64, allowedHosts []string) ([]byte, error) {
	if err := validateSourceURL(raw, allowedHosts); err != nil { return nil, err }
	client := &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 { return errors.New("too many redirects") }
		return validateSourceURL(req.URL.String(), allowedHosts)
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil); if err != nil { return nil, err }
	request.Header.Set("User-Agent", "Cherri-Shortcut-Corpus/1")
	response, err := client.Do(request); if err != nil { return nil, err }; defer response.Body.Close()
	if response.StatusCode != http.StatusOK { return nil, fmt.Errorf("HTTP %d", response.StatusCode) }
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1)); if err != nil { return nil, err }
	if int64(len(data)) > maxBytes { return nil, fmt.Errorf("source response exceeds %d-byte limit", maxBytes) }
	return data, nil
}

func validateSourceURL(raw string, allowedHosts []string) error {
	parsed, err := url.Parse(raw); if err != nil { return err }
	if parsed.Scheme != "https" { return errors.New("source URL must use HTTPS") }
	host := strings.ToLower(parsed.Hostname())
	for _, allowed := range allowedHosts { if host == strings.ToLower(allowed) { return nil } }
	return fmt.Errorf("source host %q is not allowed", host)
}
