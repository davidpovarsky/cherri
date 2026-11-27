package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The text grammar is the actual observed SentinelX continuity annotation.
// Its context identity below is synthetic; real publication originals remain
// private and are checked separately by TestDocsProcessPublicationActualOriginals.
const docsPublicationSyntheticContinuity = "SentinelX continuity: checkpoint recommended. Call sentinel_context_save(context_id='sentinelx_context_syntheticA', base_revision=11, data=<compact updated operational state>)."

func TestDocsProcessPublicationSentinelAnnotations(t *testing.T) {
	mutate := func(f *finalProvenanceFixture, id string, change func(map[string]any)) {
		o := f.object(t, id)
		change(o)
		f.put(t, id, o)
	}
	tests := []struct {
		name   string
		allow  bool
		change func(*finalProvenanceFixture)
	}{
		{"actual-continuity-shape", true, nil},
		{"duplicate-publication-result", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1] = o["content"].([]any)[0]
			})
		}},
		{"second-conflicting-publication-result", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1] = map[string]any{"type": "text", "text": `{"ok":true,"returncode":0,"output":"conflicting publication"}`}
			})
		}},
		{"structured-text-conflict", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["structuredContent"].(map[string]any)["output"] = "different actual result"
			})
		}},
		{"arbitrary-extra-text", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1].(map[string]any)["text"] = "publication succeeded"
			})
		}},
		{"duplicate-continuity-annotations", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"] = append(o["content"].([]any), o["content"].([]any)[1])
			})
		}},
		{"annotation-before-result", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				blocks := o["content"].([]any)
				blocks[0], blocks[1] = blocks[1], blocks[0]
			})
		}},
		{"annotation-without-result", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"] = o["content"].([]any)[1:]
			})
		}},
		{"nontext-annotation", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1].(map[string]any)["type"] = "resource"
			})
		}},
		{"annotation-extra-field", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1].(map[string]any)["publication"] = "success"
			})
		}},
		{"annotation-embedded-result", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1].(map[string]any)["text"] = docsPublicationSyntheticContinuity + "\n" + `{"ok":true,"returncode":0}`
			})
		}},
		{"annotation-altered-directive", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1].(map[string]any)["text"] = strings.Replace(docsPublicationSyntheticContinuity, "sentinel_context_save", "publish_source", 1)
			})
		}},
		{"annotation-overlong-context", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1].(map[string]any)["text"] = strings.Replace(docsPublicationSyntheticContinuity, "syntheticA", strings.Repeat("a", 129), 1)
			})
		}},
		{"annotation-negative-revision", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["content"].([]any)[1].(map[string]any)["text"] = strings.Replace(docsPublicationSyntheticContinuity, "base_revision=11", "base_revision=-1", 1)
			})
		}},
		{"annotated-force-command", false, func(f *finalProvenanceFixture) {
			mutate(f, "docs-push", func(o map[string]any) {
				o["command"] = append(o["command"].([]any), "--force")
			})
		}},
		{"annotated-second-ref", false, func(f *finalProvenanceFixture) {
			mutate(f, "docs-push", func(o map[string]any) {
				o["command"] = append(o["command"].([]any), "HEAD:refs/heads/other")
			})
		}},
		{"annotated-ref-mismatch", false, func(f *finalProvenanceFixture) {
			mutate(f, "docs-ref", func(o map[string]any) {
				o["object"].(map[string]any)["sha"] = strings.Repeat("f", 40)
			})
		}},
		{"annotated-failed-push", false, func(f *finalProvenanceFixture) {
			mutate(f, "docs-push", func(o map[string]any) { o["exitCode"] = 1 })
		}},
		{"annotated-provider-failure", false, func(f *finalProvenanceFixture) {
			mutate(f, "provider", func(o map[string]any) {
				o["structuredContent"].(map[string]any)["returncode"] = float64(1)
				raw, err := json.Marshal(o["structuredContent"])
				if err != nil {
					t.Fatal(err)
				}
				o["content"].([]any)[0].(map[string]any)["text"] = string(raw)
			})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := docsProcessPublicationFixture(t)
			mutate(f, "provider", func(o map[string]any) {
				o["content"] = append(o["content"].([]any), map[string]any{"type": "text", "text": docsPublicationSyntheticContinuity})
			})
			if test.change != nil {
				test.change(f)
			}
			in, err := docsProcessPublicationInputs(f)
			if err == nil {
				err = finalProvenanceValidatePublication(in, "docs")
			}
			if (err == nil) != test.allow {
				t.Fatalf("unexpected annotated original-format control result: %v", err)
			}
		})
	}
}
