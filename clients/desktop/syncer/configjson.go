package syncer

import (
	"encoding/json"
	"regexp"
	"strings"
)

var localFolderJSONPattern = regexp.MustCompile(`"(?i:local_folder|localFolder|LocalFolder)"\s*:\s*"([^"]*)"`)

// FixJSONWindowsPaths escapes single backslashes in local_folder values (common JSON mistake on Windows).
func FixJSONWindowsPaths(raw []byte) []byte {
	s := string(raw)
	fixed := localFolderJSONPattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := localFolderJSONPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		path := unescapeJSONPath(sub[1])
		return strings.Replace(match, sub[1], escapeJSONPath(path), 1)
	})
	return []byte(fixed)
}

func escapeJSONPath(path string) string {
	return strings.ReplaceAll(path, `\`, `\\`)
}

func unescapeJSONPath(path string) string {
	return strings.ReplaceAll(path, `\\`, `\`)
}

// ExtractLocalFolderFromBrokenJSON reads local_folder even when JSON has invalid escapes.
func ExtractLocalFolderFromBrokenJSON(raw []byte) (string, bool) {
	sub := localFolderJSONPattern.FindSubmatch(raw)
	if len(sub) < 2 {
		return "", false
	}
	path := strings.TrimSpace(string(sub[1]))
	if path == "" {
		return "", false
	}
	return unescapeJSONPath(path), true
}

// LoadConfigJSON unmarshals after fixing common Windows path escaping in local_folder.
func LoadConfigJSON[T any](raw []byte, dst *T) error {
	fixed := FixJSONWindowsPaths(raw)
	if err := json.Unmarshal(fixed, dst); err == nil {
		return nil
	}
	return json.Unmarshal(raw, dst)
}
