package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

// This format consumes the actual preflight and process originals. It does not
// convert them to docs-main-push/1 or infer an unrecorded attempt counter.
const docsProcessPublicationFormat = "docs-original-process-push/1"

// SentinelX may append one non-authoritative continuity reminder after its
// result. This closed grammar cannot carry another publication result.
var docsPublicationContinuityPattern = regexp.MustCompile("^SentinelX continuity: checkpoint recommended\\. Call sentinel_context_save\\(context_id='sentinelx_context_[A-Za-z0-9_]{1,128}', base_revision=(0|[1-9][0-9]{0,9}), data=<compact updated operational state>\\)\\.$")

func docsPublicationContinuityAnnotation(value any) bool {
	note, ok := value.(map[string]any)
	if !ok || len(note) != 2 || finalProvenanceString(note, "type") != "text" {
		return false
	}
	text, ok := note["text"].(string)
	return ok && len(text) <= 512 && docsPublicationContinuityPattern.MatchString(text)
}

var docsPublicationPostNames = []string{
	"post-head", "post-tree", "post-ref", "post-status", "post-diff",
	"post-origin", "post-push-origin", "post-base", "post-index", "post-replace",
}

func docsPublicationPostCommands(root, previous, source string) map[string][]string {
	prefix := []string{"/usr/bin/git", "--no-replace-objects", "-C", root, "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.ignoreStat=false"}
	queries := [][]string{
		{"rev-parse", "HEAD"}, {"rev-parse", "HEAD^{tree}"}, {"symbolic-ref", "-q", "HEAD"},
		{"status", "--porcelain=v1", "--untracked-files=all", "--ignore-submodules=none"},
		{"diff", "--name-only", "HEAD", "--"}, {"remote", "get-url", "--all", "origin"},
		{"remote", "get-url", "--push", "--all", "origin"},
		{"merge-base", "--is-ancestor", previous, source}, {"ls-files", "-v", "-z"},
		{"for-each-ref", "--format=%(refname)", "refs/replace/"},
	}
	commands := map[string][]string{}
	for i, name := range docsPublicationPostNames {
		commands[name] = append(append([]string{}, prefix...), queries[i]...)
	}
	return commands
}

func docsPublicationProcessStreams(in *finalProvenanceInputs, step map[string]any, pair finalProvenanceStreams) error {
	for field, id := range map[string]string{"stdout": pair.Stdout, "stderr": pair.Stderr} {
		pin, ok := in.Expected.Files[id]
		b, retained := in.Bytes[id]
		if id == "" || !ok || !retained || int64(len(b)) != pin.Bytes ||
			finalProvenanceDigest(b) != pin.SHA256 || finalProvenanceString(step, field+"Sha256") != pin.SHA256 {
			return fmt.Errorf("docs process stream is not bound to the original bytes")
		}
	}
	return nil
}

func finalProvenanceDocsProcessPublication(in *finalProvenanceInputs, p finalProvenancePublication, s finalProvenanceSource, preflight map[string]any) error {
	fail := func(detail string) error { return fmt.Errorf("docs original process publication %s", detail) }
	root := in.Manifest.SourceDirectories["docs"]
	previous := finalProvenanceString(preflight, "previousMain")
	if s.Repository != "davidpovarsky/cherrilang.org" || s.PublishedRef != "refs/heads/main" ||
		!filepath.IsAbs(root) || filepath.Clean(root) != root || p.PushStreams == nil ||
		p.PublisherRef != "" || p.Reconciliation != nil ||
		finalProvenanceString(preflight, "schema") != "cherri.docs-publication-preflight/v1" ||
		finalProvenanceString(preflight, "docsCommit") != s.SHA ||
		finalProvenanceString(preflight, "compilerCommit") != in.Expected.Sources["compiler"].SHA ||
		!finalProvenanceHex(previous, 40) || previous == s.SHA ||
		!finalProvenanceBool(preflight, "normalFastForwardOnly", true) ||
		!finalProvenanceBool(preflight, "cleanDocsSource", true) ||
		!finalProvenanceBool(preflight, "compilerMainOr179AcceptanceGranted", false) {
		return fail("source, preflight or scope differs")
	}
	if len(p.Receipts) != 6 || len(p.Streams) != len(docsPublicationPostNames) {
		return fail("original receipt or post-source stream inventory is incomplete")
	}
	seen := map[string]bool{}
	for _, id := range []string{p.Result, p.Push, p.Ref, p.Commit, p.PushStreams.Stdout, p.PushStreams.Stderr} {
		if id == "" || seen[id] {
			return fail("original identity is absent or replayed")
		}
		seen[id] = true
	}
	for _, id := range p.Receipts {
		if id == "" || seen[id] {
			return fail("original receipt identity is absent or replayed")
		}
		seen[id] = true
	}
	originals := finalProvenanceObject(preflight, "sourceOriginals")
	if len(originals) != 4 {
		return fail("preflight original inventory differs")
	}
	for _, name := range []string{"docsIntake", "docsRun", "s4Intake", "s4Handoff"} {
		id := p.Receipts["preflight."+name]
		row := finalProvenanceObject(originals, name)
		pin, ok := in.Expected.Files[id]
		if !ok || len(row) != 2 || finalProvenanceString(row, "sha256") != pin.SHA256 ||
			finalProvenanceString(row, "path") != in.Manifest.Files[id] {
			return fail("preflight original is missing or differs: " + name)
		}
	}
	push, err := in.object(p.Push)
	if err != nil {
		return err
	}
	argv, ok := finalProvenanceStrings(push["command"])
	want := []string{"sudo", "-u", "ubuntu", "git", "-C", root,
		"-c", "remote.origin.mirror=false", "-c", "credential.helper=",
		"-c", "credential.helper=!gh auth git-credential", "push", "origin", s.SHA + ":" + s.PublishedRef}
	code, codeOK := finalProvenanceNumber(push, "exitCode")
	if !ok || !reflect.DeepEqual(argv, want) || !codeOK || code != 0 {
		return fail("exact nonforce single-ref push command or outcome differs")
	}
	if err = docsPublicationProcessStreams(in, push, *p.PushStreams); err != nil {
		return err
	}
	preAt, e1 := finalProvenanceTime(finalProvenanceString(preflight, "atUtc"))
	pushStart, e2 := finalProvenanceTime(finalProvenanceString(push, "startedUtc"))
	pushEnd, e3 := finalProvenanceTime(finalProvenanceString(push, "finishedUtc"))
	if e1 != nil || e2 != nil || e3 != nil || pushStart.Before(preAt) || pushEnd.Before(pushStart) {
		return fail("preflight and push chronology differs")
	}
	origin := "https://github.com/" + s.Repository + ".git"
	stderr := string(in.Bytes[p.PushStreams.Stderr])
	matches := regexp.MustCompile("^To " + regexp.QuoteMeta(origin) + "\n   ([0-9a-f]{7,40})\\.\\.([0-9a-f]{7,40})  " + s.SHA + " -> main\n$").FindStringSubmatch(stderr)
	if len(in.Bytes[p.PushStreams.Stdout]) != 0 || len(matches) != 3 ||
		!strings.HasPrefix(previous, matches[1]) || !strings.HasPrefix(s.SHA, matches[2]) {
		return fail("actual fast-forward push output differs")
	}
	provider, err := in.object(p.Receipts["provider_response"])
	if err != nil {
		return err
	}
	structured := finalProvenanceObject(provider, "structuredContent")
	providerCode, providerCodeOK := finalProvenanceNumber(structured, "returncode")
	if !finalProvenanceBool(provider, "isError", false) || !finalProvenanceBool(structured, "ok", true) || !providerCodeOK || providerCode != 0 {
		return fail("original provider response did not succeed")
	}
	content := finalProvenanceArray(provider, "content")
	if len(content) < 1 || len(content) > 2 {
		return fail("provider response content is ambiguous")
	}
	if len(content) == 2 && !docsPublicationContinuityAnnotation(content[1]) {
		return fail("provider response has an unrecognized annotation or second result")
	}
	block, ok := content[0].(map[string]any)
	if !ok || finalProvenanceString(block, "type") != "text" {
		return fail("provider response content is not the original text")
	}
	contentObject, err := decodeNativeEncodingObject([]byte(finalProvenanceString(block, "text")))
	if err != nil || !reflect.DeepEqual(contentObject, structured) {
		return fail("provider structured and textual originals differ")
	}
	summary, err := decodeNativeEncodingObject([]byte(finalProvenanceString(structured, "output")))
	summaryCode, summaryCodeOK := finalProvenanceNumber(summary, "exitCode")
	if err != nil || !summaryCodeOK || summaryCode != 0 ||
		finalProvenanceString(summary, "preflightPath") != in.Manifest.Files[p.Result] ||
		finalProvenanceString(summary, "preflightSha256") != in.Expected.Files[p.Result].SHA256 ||
		finalProvenanceString(summary, "stderr") != stderr ||
		finalProvenanceString(summary, "afterRemoteMain") != s.SHA+"\t"+s.PublishedRef {
		return fail("immediate provider readback differs from the original push")
	}
	ref, err := in.object(p.Ref)
	if err != nil {
		return err
	}
	commit, err := in.object(p.Commit)
	if err != nil {
		return err
	}
	object := finalProvenanceObject(ref, "object")
	if finalProvenanceString(ref, "ref") != s.PublishedRef ||
		finalProvenanceString(ref, "url") != "https://api.github.com/repos/"+s.Repository+"/git/refs/heads/main" ||
		finalProvenanceString(object, "sha") != s.SHA || finalProvenanceString(object, "type") != "commit" ||
		finalProvenanceString(commit, "sha") != s.SHA || finalProvenanceString(finalProvenanceObject(commit, "tree"), "sha") != s.Tree {
		return fail("authenticated remote ref or commit differs")
	}
	witness, err := in.object(p.Receipts["source_after"])
	if err != nil {
		return err
	}
	start, e1 := finalProvenanceTime(finalProvenanceString(witness, "startedUtc"))
	end, e2 := finalProvenanceTime(finalProvenanceString(witness, "finishedUtc"))
	if e1 != nil || e2 != nil || start.Before(pushEnd) || end.Before(start) ||
		finalProvenanceString(witness, "schema") != "cherri-docs-publication-source-observation/1" ||
		finalProvenanceString(witness, "sourceDirectory") != root ||
		finalProvenanceString(witness, "sourceCommit") != s.SHA || finalProvenanceString(witness, "sourceTree") != s.Tree ||
		finalProvenanceString(witness, "localRef") != s.LocalRef || finalProvenanceString(witness, "previousMain") != previous {
		return fail("post-push source observation identity or chronology differs")
	}
	steps := finalProvenanceArray(witness, "commands")
	commands := docsPublicationPostCommands(root, previous, s.SHA)
	if len(steps) != len(docsPublicationPostNames) {
		return fail("post-source command inventory is incomplete")
	}
	outputs := map[string]string{}
	previousEnd := start
	for i, name := range docsPublicationPostNames {
		row, ok := steps[i].(map[string]any)
		argv, argvOK := finalProvenanceStrings(row["argv"])
		code, codeOK := finalProvenanceNumber(row, "exitCode")
		started, e1 := finalProvenanceTime(finalProvenanceString(row, "startedUtc"))
		finished, e2 := finalProvenanceTime(finalProvenanceString(row, "finishedUtc"))
		pair, exists := p.Streams[name]
		if !ok || !argvOK || !reflect.DeepEqual(argv, commands[name]) || !codeOK || code != 0 ||
			!finalProvenanceBool(row, "timedOut", false) || finalProvenanceString(row, "label") != name ||
			e1 != nil || e2 != nil || started.Before(previousEnd) || finished.Before(started) || finished.After(end) || !exists {
			return fail("post-source command or original outcome differs: " + name)
		}
		for _, id := range []string{pair.Stdout, pair.Stderr} {
			if id == "" || seen[id] {
				return fail("post-source stream identity is absent or replayed")
			}
			seen[id] = true
		}
		if err := docsPublicationProcessStreams(in, row, pair); err != nil {
			return err
		}
		if len(in.Bytes[pair.Stderr]) != 0 {
			return fail("post-source command produced an unexpected error stream")
		}
		outputs[name] = string(in.Bytes[pair.Stdout])
		previousEnd = finished
	}
	for name, want := range map[string]string{
		"post-head": s.SHA + "\n", "post-tree": s.Tree + "\n", "post-ref": s.LocalRef + "\n",
		"post-status": "", "post-diff": "", "post-origin": origin + "\n", "post-push-origin": origin + "\n",
		"post-base": "", "post-replace": "",
	} {
		if outputs[name] != want {
			return fail("post-source original does not prove the expected state: " + name)
		}
	}
	if outputs["post-index"] == "" {
		return fail("source index inventory is absent")
	}
	for _, row := range strings.Split(outputs["post-index"], "\x00") {
		if row != "" && (len(row) < 3 || row[:2] != "H ") {
			return fail("source index hides or changes tracked content")
		}
	}
	return nil
}
