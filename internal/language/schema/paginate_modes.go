package schema

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The native Number comparison and URL-text path share the exact integer range.
// Values beyond it are not silently rounded into a different request marker.
const PaginateMaxMarker int64 = 9007199254740991

var paginateIntegerMarker = regexp.MustCompile("^(0|[1-9][0-9]*)$")

func resolvePaginateMode(out *PaginateOptions, params map[string]any, parsed *url.URL) error {
	out.Mode = "url"
	if value, present := params["WFPaginateMode"]; present {
		mode, literal := value.(string)
		if !literal {
			return fmt.Errorf("PAGINATE_MODE: mode must be a literal url, cursor, page or offset")
		}
		out.Mode = mode
	}
	switch out.Mode {
	case "url":
		if _, present := params["WFPaginateQueryKey"]; present {
			return fmt.Errorf("PAGINATE_QUERY_KEY: queryKey is valid only for cursor, page or offset mode")
		}
		return nil
	case "cursor", "page", "offset":
	default:
		return fmt.Errorf("PAGINATE_MODE: mode must be url, cursor, page or offset")
	}
	out.QueryKey = out.Mode
	if value, present := params["WFPaginateQueryKey"]; present {
		key, literal := value.(string)
		if !literal {
			return fmt.Errorf("PAGINATE_QUERY_KEY: queryKey must be literal Text")
		}
		out.QueryKey = key
	}
	if out.QueryKey == "" || len(out.QueryKey) > 128 || !utf8.ValidString(out.QueryKey) ||
		strings.IndexFunc(out.QueryKey, unicode.IsControl) >= 0 {
		return fmt.Errorf("PAGINATE_QUERY_KEY: queryKey must be 1..128 UTF-8 bytes without control characters")
	}
	// Parse for validity, but do not serialize Values back into the request:
	// reordering keys or changing percent escapes can alter an authored URL.
	if _, err := url.ParseQuery(parsed.RawQuery); err != nil {
		return fmt.Errorf("PAGINATE_QUERY: the authored query must have valid escaped key/value syntax")
	}
	base := *parsed
	base.RawQuery, base.ForceQuery = "", false
	query := parsed.RawQuery
	position, matches := 0, 0
	for _, part := range strings.Split(query, "&") {
		rawKey, rawValue, _ := strings.Cut(part, "=")
		key, err := url.QueryUnescape(rawKey)
		if err != nil {
			return fmt.Errorf("PAGINATE_QUERY: query key has invalid escapes")
		}
		if key == out.QueryKey {
			matches++
			if matches > 1 {
				return fmt.Errorf("PAGINATE_QUERY: the configured marker key must occur at most once")
			}
			marker, err := url.QueryUnescape(rawValue)
			if err != nil || !utf8.ValidString(marker) {
				return fmt.Errorf("PAGINATE_QUERY: the initial marker must be valid UTF-8")
			}
			out.InitialMarker = marker
			out.HasInitialMarker = marker != ""
			out.QueryPrefix = base.String() + "?" + query[:position] + rawKey + "="
			out.QuerySuffix = query[position+len(part):]
		}
		position += len(part) + 1
	}
	if matches == 0 {
		prefix := base.String() + "?"
		if query != "" {
			prefix += query
			if !strings.HasSuffix(query, "&") {
				prefix += "&"
			}
		}
		out.QueryPrefix = prefix + strings.ReplaceAll(url.QueryEscape(out.QueryKey), "+", "%20") + "="
	}
	if out.Mode == "page" || out.Mode == "offset" {
		if matches != 1 || !paginateIntegerMarker.MatchString(out.InitialMarker) {
			return fmt.Errorf("PAGINATE_INITIAL_MARKER: numeric modes require one canonical integer marker in startURL")
		}
		number, err := strconv.ParseUint(out.InitialMarker, 10, 64)
		if err != nil || number > uint64(PaginateMaxMarker) || (out.Mode == "page" && number == 0) {
			return fmt.Errorf("PAGINATE_INITIAL_MARKER: page markers must be 1..9007199254740991 and offset markers 0..9007199254740991")
		}
		out.HasInitialMarker = true
	}
	return nil
}
