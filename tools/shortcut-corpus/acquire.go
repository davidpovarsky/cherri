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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/electrikmilk/cherri/internal/icloudshortcut"
)

const acquisitionStateVersion = 1

type sourcePolicy string

const (
	policyOfficialAPI sourcePolicy = "OFFICIAL_API"
	policyPublicFeed  sourcePolicy = "PUBLIC_MACHINE_FEED"
	policyBootstrap   sourcePolicy = "BOOTSTRAP_DATASET"
	policyManualOnly  sourcePolicy = "MANUAL_ONLY"
	policyDisabled    sourcePolicy = "DISABLED"
)

type sourceInfo struct {
	Name string
	Policy sourcePolicy
	EnabledByDefault bool
	Reference string
	Reason string
}

var sourceRegistry = []sourceInfo{
	{Name: "seed", Policy: policyPublicFeed, EnabledByDefault: true, Reason: "User-supplied public iCloud share links; deterministic fallback."},
	{Name: "routinehub", Policy: policyPublicFeed, EnabledByDefault: true, Reference: "https://routinehub.co/docs/", Reason: "Public discovery feed/page plus public Shortcut share links; isolated adapter."},
	{Name: "shortcutsbench", Policy: policyBootstrap, EnabledByDefault: false, Reference: "https://github.com/EachSheep/ShortcutsBench", Reason: "Apache-2.0 historical bootstrap; minimal iCloud/provenance fields only."},
	{Name: "github-public", Policy: policyDisabled, EnabledByDefault: false, Reference: "https://docs.github.com/en/rest/search/search", Reason: "Optional future long-tail source; disabled until a public-only authenticated search adapter is configured."},
	{Name: "macstories", Policy: policyManualOnly, Reference: "https://www.macstories.net/terms-of-service/", Reason: "Shortcut archive is useful, but general site terms restrict automated content access/data mining."},
	{Name: "matthew-free", Policy: policyManualOnly, Reference: "https://matthewcassinelli.com/sirishortcuts/library/free/", Reason: "Free library is useful; no default automation permission is assumed."},
	{Name: "shareshortcuts", Policy: policyManualOnly, Reference: "https://shareshortcuts.com/terms.php", Reason: "Current terms prohibit automated agents/scripts generating automated requests/searches."},
	{Name: "shortcutsgallery", Policy: policyManualOnly, Reference: "https://shortcutsgallery.com/terms-of-use/", Reason: "Restrictive site-use terms; use independently supplied iCloud links instead."},
}

type discoveredShortcut struct {
	Source string `json:"source"`
	SourceItemID string `json:"sourceItemId,omitempty"`
	SourceItemURL string `json:"sourceItemUrl,omitempty"`
	Version string `json:"version,omitempty"`
	ICloudURL string `json:"icloudUrl"`
}

type acquisitionItem struct {
	Source string `json:"source"`
	SourceItemID string `json:"sourceItemId,omitempty"`
	Version string `json:"version,omitempty"`
	ICloudID string `json:"icloudId,omitempty"`
	ICloudURL string `json:"icloudUrl,omitempty"`
	ContentSHA256 string `json:"contentSha256,omitempty"`
	LastSeen string `json:"lastSeen,omitempty"`
	LastFetched string `json:"lastFetched,omitempty"`
	Status string `json:"status,omitempty"`
	Error string `json:"error,omitempty"`
}

type acquisitionContent struct {
	SHA256 string `json:"sha256"`
	ByteSize int `json:"byteSize"`
	FirstSeen string `json:"firstSeen"`
	Sources []string `json:"sources,omitempty"`
	ICloudIDs []string `json:"icloudIds,omitempty"`
	Path string `json:"path,omitempty"`
}

type acquisitionState struct {
	Version int `json:"version"`
	Items map[string]*acquisitionItem `json:"items"`
	ICloud map[string]string `json:"icloud"`
	Content map[string]*acquisitionContent `json:"content"`
	Sources map[string]string `json:"sources,omitempty"`
}

type collectConfig struct {
	Sources string
	SeedPath string
	Inbox string
	StatePath string
	MaxItems int
	DryRun bool
	RoutineHubFeed string
	ShortcutsBenchURL string
}

type collectStats struct {
	Discovered int `json:"discovered"`
	KnownItems int `json:"knownItems"`
	CanonicalICloud int `json:"canonicalICloud"`
	Downloaded int `json:"downloaded"`
	DuplicateContent int `json:"duplicateContent"`
	Failed int `json:"failed"`
	DistinctContent int `json:"distinctContent"`
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
	dir := filepath.Dir(path); if dir != "." { if err := os.MkdirAll(dir, 0755); err != nil { return err } }
	encoded, err := json.MarshalIndent(state, "", "  "); if err != nil { return err }
	temp := path + ".tmp"; if err = os.WriteFile(temp, append(encoded, '\n'), 0600); err != nil { return err }; return os.Rename(temp, path)
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
	flags.StringVar(&config.RoutineHubFeed, "routinehub-feed", envOr("ROUTINEHUB_FEED_URL", "https://routinehub.co/shortcuts/latest/feed/"), "RoutineHub public discovery feed/listing")
	flags.StringVar(&config.ShortcutsBenchURL, "shortcutsbench-url", envOr("SHORTCUTSBENCH_DATA_URL", "https://raw.githubusercontent.com/EachSheep/ShortcutsBench/master/deves_dataset/dataset_src/routinehub.co.json"), "ShortcutsBench JSON dataset URL")
}

func runCollect(argv []string) {
	var config collectConfig; flags := flag.NewFlagSet("collect", flag.ExitOnError); addCollectFlags(flags, &config); flags.Parse(argv)
	if flags.NArg() != 0 { fmt.Fprint(os.Stderr, usage); os.Exit(2) }
	stats, err := collect(config)
	if err != nil { fmt.Fprintf(os.Stderr, "Error: %v\n", err); os.Exit(1) }
	fmt.Printf("Collected: discovered=%d known=%d downloaded=%d duplicate-content=%d distinct-content=%d failed=%d\n", stats.Discovered, stats.KnownItems, stats.Downloaded, stats.DuplicateContent, stats.DistinctContent, stats.Failed)
}

func collect(config collectConfig) (collectStats, error) {
	state, err := loadAcquisitionState(config.StatePath); if err != nil { return collectStats{}, err }
	ctx := context.Background(); var discovered []discoveredShortcut
	for _, source := range splitSources(config.Sources) {
		var items []discoveredShortcut; var discoverErr error
		switch source {
		case "seed": items, discoverErr = discoverSeed(config.SeedPath)
		case "routinehub": items, discoverErr = discoverRoutineHub(ctx, config.RoutineHubFeed)
		case "shortcutsbench": items, discoverErr = discoverShortcutsBench(ctx, config.ShortcutsBenchURL)
		default: discoverErr = fmt.Errorf("source %q is not enabled for automated collection; use a seed file with public iCloud links", source)
		}
		if discoverErr != nil { state.Sources[source] = "error: " + discoverErr.Error(); continue }
		state.Sources[source] = "ok " + now(); discovered = append(discovered, items...)
	}
	discovered = dedupeDiscovered(discovered)
	stats := collectStats{Discovered: len(discovered)}
	resolver := icloudshortcut.NewResolver()
	processed := 0
	for _, item := range discovered {
		if config.MaxItems > 0 && processed >= config.MaxItems { break }
		canonical, id, canonicalErr := icloudshortcut.CanonicalURL(item.ICloudURL)
		if canonicalErr != nil { stats.Failed++; continue }
		item.ICloudURL = canonical
		key := item.Source + ":" + item.SourceItemID
		if item.SourceItemID == "" { key = item.Source + ":" + id }
		if existing := state.Items[key]; existing != nil && existing.ICloudID == id && existing.Version == item.Version && existing.ContentSHA256 != "" {
			existing.LastSeen = now(); stats.KnownItems++; continue
		}
		if contentSHA := state.ICloud[id]; contentSHA != "" {
			state.Items[key] = &acquisitionItem{Source: item.Source, SourceItemID: item.SourceItemID, Version: item.Version, ICloudID: id, ICloudURL: canonical, ContentSHA256: contentSHA, LastSeen: now(), Status: "known"}
			if content := state.Content[contentSHA]; content != nil { content.Sources = boundedUniqueStrings(append(content.Sources, item.Source), 32); content.ICloudIDs = boundedUniqueStrings(append(content.ICloudIDs, id), 32) }
			stats.KnownItems++; continue
		}
		processed++
		if config.DryRun { continue }
		result, resolveErr := resolver.Resolve(ctx, canonical)
		if resolveErr != nil {
			state.Items[key] = &acquisitionItem{Source: item.Source, SourceItemID: item.SourceItemID, Version: item.Version, ICloudID: id, ICloudURL: canonical, LastSeen: now(), Status: "failed", Error: resolveErr.Error()}
			stats.Failed++; continue
		}
		if _, parseErr := parseDocument(id+".plist", result.Data); parseErr != nil {
			state.Items[key] = &acquisitionItem{Source: item.Source, SourceItemID: item.SourceItemID, Version: item.Version, ICloudID: id, ICloudURL: canonical, LastSeen: now(), Status: "invalid", Error: parseErr.Error()}
			stats.Failed++; continue
		}
		sha := sha256.Sum256(result.Data); digest := hex.EncodeToString(sha[:]); state.ICloud[id] = digest
		content := state.Content[digest]
		if content == nil {
			dir := filepath.Join(config.Inbox, item.Source); if err = os.MkdirAll(dir, 0700); err != nil { return stats, err }
			path := filepath.Join(dir, digest+".plist"); if err = os.WriteFile(path, result.Data, 0600); err != nil { return stats, err }
			content = &acquisitionContent{SHA256: digest, ByteSize: len(result.Data), FirstSeen: now(), Sources: []string{item.Source}, ICloudIDs: []string{id}, Path: path}; state.Content[digest] = content; stats.DistinctContent++
		} else { content.Sources = boundedUniqueStrings(append(content.Sources, item.Source), 32); content.ICloudIDs = boundedUniqueStrings(append(content.ICloudIDs, id), 32); stats.DuplicateContent++ }
		state.Items[key] = &acquisitionItem{Source: item.Source, SourceItemID: item.SourceItemID, Version: item.Version, ICloudID: id, ICloudURL: canonical, ContentSHA256: digest, LastSeen: now(), LastFetched: now(), Status: "ok"}; stats.Downloaded++
	}
	if err = state.save(config.StatePath); err != nil { return stats, err }
	return stats, nil
}

var icloudLinkPattern = regexp.MustCompile(`https://(?:www\.)?icloud\.com/shortcuts/[A-Za-z0-9]{16,64}`)
var routineHubShortcutPattern = regexp.MustCompile(`https://routinehub\.co/shortcut/([0-9]+)[^\s"'<>]*`)

func discoverSeed(path string) ([]discoveredShortcut, error) {
	if path == "" { return nil, errors.New("seed source requires -seed <file>") }
	data, err := os.ReadFile(path); if err != nil { return nil, err }
	return linksToDiscovered("seed", string(data)), nil
}

func discoverRoutineHub(ctx context.Context, feed string) ([]discoveredShortcut, error) {
	body, err := fetchSource(ctx, feed, 4<<20); if err != nil { return nil, err }
	items := linksToDiscovered("routinehub", string(body)); if len(items) != 0 { return items, nil }
	ids := routineHubShortcutPattern.FindAllStringSubmatch(string(body), -1); seen := map[string]bool{}
	for _, match := range ids {
		id := match[1]; if seen[id] { continue }; seen[id] = true
		pageURL := "https://routinehub.co/shortcut/" + id + "/"
		page, pageErr := fetchSource(ctx, pageURL, 2<<20); if pageErr != nil { continue }
		for _, found := range linksToDiscovered("routinehub", string(page)) { found.SourceItemID = id; found.SourceItemURL = pageURL; items = append(items, found) }
		if len(items) >= 100 { break }
	}
	if len(items) == 0 { return nil, errors.New("RoutineHub discovery surface yielded no public iCloud links") }
	return items, nil
}

func discoverShortcutsBench(ctx context.Context, rawURL string) ([]discoveredShortcut, error) {
	body, err := fetchSource(ctx, rawURL, 64<<20); if err != nil { return nil, err }
	var records []map[string]any
	if err = json.Unmarshal(body, &records); err != nil { return nil, fmt.Errorf("ShortcutsBench dataset JSON: %w", err) }
	var items []discoveredShortcut
	for index, record := range records {
		raw, _ := record["URL"].(string); if !icloudLinkPattern.MatchString(raw) { continue }
		source, _ := record["Source"].(string); if source == "" { source = "shortcutsbench" }
		items = append(items, discoveredShortcut{Source: "shortcutsbench", SourceItemID: fmt.Sprintf("%d", index), SourceItemURL: source, ICloudURL: icloudLinkPattern.FindString(raw)})
	}
	return items, nil
}

func linksToDiscovered(source, text string) []discoveredShortcut {
	links := icloudLinkPattern.FindAllString(text, -1); var items []discoveredShortcut
	for index, link := range links { items = append(items, discoveredShortcut{Source: source, SourceItemID: fmt.Sprintf("%d", index), ICloudURL: link}) }
	return items
}

func dedupeDiscovered(items []discoveredShortcut) []discoveredShortcut {
	seen := map[string]bool{}; var result []discoveredShortcut
	for _, item := range items { canonical, id, err := icloudshortcut.CanonicalURL(item.ICloudURL); if err != nil || seen[id] { continue }; seen[id] = true; item.ICloudURL = canonical; result = append(result, item) }
	sort.Slice(result, func(i, j int) bool { return result[i].ICloudURL < result[j].ICloudURL }); return result
}

func splitSources(raw string) []string { parts := strings.Split(raw, ","); var result []string; for _, part := range parts { part = strings.TrimSpace(part); if part != "" { result = append(result, part) } }; return result }
func envOr(key, fallback string) string { if value := os.Getenv(key); value != "" { return value }; return fallback }

func fetchSource(ctx context.Context, raw string, maxBytes int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil); if err != nil { return nil, err }; request.Header.Set("User-Agent", "Cherri-Shortcut-Corpus/1")
	client := &http.Client{Timeout: 25 * time.Second}; response, err := client.Do(request); if err != nil { return nil, err }; defer response.Body.Close()
	if response.StatusCode != http.StatusOK { return nil, fmt.Errorf("HTTP %d", response.StatusCode) }
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1)); if err != nil { return nil, err }; if int64(len(data)) > maxBytes { return nil, fmt.Errorf("source response exceeds %d-byte limit", maxBytes) }; return data, nil
}
