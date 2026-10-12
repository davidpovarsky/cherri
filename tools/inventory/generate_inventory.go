package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
)

type CatalogOutput struct {
	OK      bool            `json:"ok"`
	Version string          `json:"version"`
	Count   int             `json:"count"`
	Actions []CatalogAction `json:"actions"`
}

type CatalogAction struct {
	Name               string             `json:"name"`
	ShortcutIdentifier string             `json:"shortcutIdentifier"`
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	Category           string             `json:"category"`
	Subcategory        string             `json:"subcategory"`
	Parameters         []CatalogParameter `json:"parameters"`
	EmittedKeys        []string           `json:"emittedKeys"`
	OutputType         string             `json:"outputType"`
	MacOnly            bool               `json:"macOnly,omitempty"`
	NonMacOnly         bool               `json:"nonMacOnly,omitempty"`
	MinVersion         float64            `json:"minVersion,omitempty"`
	MaxVersion         float64            `json:"maxVersion,omitempty"`
	Builtin            bool               `json:"builtin,omitempty"`
	Custom             bool               `json:"custom,omitempty"`
	CompilerConstruct  bool               `json:"compilerConstruct,omitempty"`
	InsertionSnippet   string             `json:"insertionSnippet,omitempty"`
	AppIntent          *CatalogAppIntent  `json:"appIntent,omitempty"`
	StaticParameters   map[string]any     `json:"staticParameters,omitempty"`
}

type CatalogParameter struct {
	Name       string   `json:"name"`
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Optional   bool     `json:"optional,omitempty"`
	Infinite   bool     `json:"infinite,omitempty"`
	Reference  bool     `json:"reference,omitempty"`
	Literal    bool     `json:"literal,omitempty"`
	Enum       string   `json:"enum,omitempty"`
	EnumValues []string `json:"enumValues,omitempty"`
	Default    string   `json:"default,omitempty"`
}

type CatalogAppIntent struct {
	Name                string `json:"name"`
	BundleIdentifier    string `json:"bundleIdentifier"`
	AppIntentIdentifier string `json:"appIntentIdentifier"`
	TeamIdentifier      string `json:"teamIdentifier,omitempty"`
}

type VariantGroup struct {
	AppleIdentifier string          `json:"appleIdentifier"`
	VariantCount    int             `json:"variantCount"`
	Callables       []string        `json:"callables"`
	Actions         []CatalogAction `json:"actions"`
}

type ActionDelta struct {
	ForkAddedActions       []string `json:"forkAddedActions"`
	ForkRelocatedActions   []string `json:"forkRelocatedActions"`
	ForkModifiedActions    []string `json:"forkModifiedActions"`
	UpstreamRemovedActions []string `json:"upstreamRemovedActions"`
	Summary                struct {
		TotalCatalogCallables   int `json:"totalCatalogCallables"`
		TotalUniqueAppleIDs     int `json:"totalUniqueAppleIDs"`
		MultiCallableIdentifierGroups int `json:"multiCallableIdentifierGroups"`
		AddedDeclarativeCount   int `json:"addedDeclarativeCount"`
		AddedStdActionsCount    int `json:"addedStdActionsCount"`
		RelocatedToStdCount     int `json:"relocatedToStdCount"`
		ModifiedActionCount     int `json:"modifiedActionCount"`
	} `json:"summary"`
}

func main() {
	cmd := exec.Command("go", "run", ".", "--actions-json")
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run cherri --actions-json: %v\n", err)
		os.Exit(1)
	}

	var cat CatalogOutput
	if err := json.Unmarshal(out, &cat); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse catalog json: %v\n", err)
		os.Exit(1)
	}

	// 1. Known action variants
	byAppleID := make(map[string][]CatalogAction)
	for _, act := range cat.Actions {
		ident := act.ShortcutIdentifier
		if ident == "" {
			ident = "compiler.construct"
		}
		byAppleID[ident] = append(byAppleID[ident], act)
	}

	var multiGroups []VariantGroup
	for ident, acts := range byAppleID {
		if len(acts) > 1 {
			names := make([]string, len(acts))
			for i, a := range acts {
				names[i] = a.Name
			}
			sort.Strings(names)
			multiGroups = append(multiGroups, VariantGroup{
				AppleIdentifier: ident,
				VariantCount:    len(acts),
				Callables:       names,
				Actions:         acts,
			})
		}
	}

	sort.Slice(multiGroups, func(i, j int) bool {
		return multiGroups[i].AppleIdentifier < multiGroups[j].AppleIdentifier
	})

	variantsData, err := json.MarshalIndent(multiGroups, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("docs/architecture/known-action-variants.json", variantsData, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Wrote docs/architecture/known-action-variants.json (%d multi-action groups)\n", len(multiGroups))

	// 2. Action definition delta
	delta := ActionDelta{
		ForkAddedActions: []string{
			// 13 declarative in actions/*.cherri
			"getVariable", "getType",
			"getUpcomingEvents", "getTimeBetweenDates", "addEvent", "addReminder",
			"getDirections", "getTravelTime", "getDistance",
			"searchiTunes", "createPlaylist",
			"selectContacts",
			"getAppStoreDetail",
			// 7 added in actions_std.go
			"setSilentMode", "searchSpotlight", "createRemindersList",
			"startStopwatch", "stopStopwatch", "playAudiobook", "openBook",
		},
		ForkRelocatedActions: []string{
			"run", "transcribeText",
		},
		ForkModifiedActions: []string{
			"runJavaScriptOnWebpage", "saveFile", "openApp", "base64Encode", "base64Decode",
			"combineImages", "getFileDetail", "calculate", "prompt", "filterFiles",
		},
		UpstreamRemovedActions: []string{}, // NONE removed
	}

	sort.Strings(delta.ForkAddedActions)
	sort.Strings(delta.ForkRelocatedActions)
	sort.Strings(delta.ForkModifiedActions)

	delta.Summary.TotalCatalogCallables = len(cat.Actions)
	delta.Summary.TotalUniqueAppleIDs = len(byAppleID)
	delta.Summary.MultiCallableIdentifierGroups = len(multiGroups)
	delta.Summary.AddedDeclarativeCount = 13
	delta.Summary.AddedStdActionsCount = 7
	delta.Summary.RelocatedToStdCount = 2
	delta.Summary.ModifiedActionCount = len(delta.ForkModifiedActions)

	deltaData, err := json.MarshalIndent(delta, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("docs/architecture/action-definition-delta.json", deltaData, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Wrote docs/architecture/action-definition-delta.json\n")

	if err := generateOwnership(); err != nil {
		panic(err)
	}
}
