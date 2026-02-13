package download

import "strings"

var replacer = strings.NewReplacer(
	":", "_",
	"/", "_",
	"<", "_",
	">", "_",
	"'", "_",
	"\\", "_",
	"|", "_",
	"?", "_",
	"*", "_",
	" ", "_",
)

// MakeValid normalizes names into filesystem-safe filenames.
func MakeValid(name string) string {
	return replacer.Replace(name)
}
