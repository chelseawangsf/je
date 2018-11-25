package je

import (
	"net/url"
	"strconv"
	"strings"
)

// SafeParseInt ...
func SafeParseInt(s string, d int) int {
	n, e := strconv.Atoi(s)
	if e != nil {
		return d
	}
	return n
}

// SafeParseUint64 ...
func SafeParseUint64(s string, d uint64) uint64 {
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		return d
	}
	return n
}

// JoinArgs ...
func JoinArgs(args []string) string {
	return url.QueryEscape(strings.Join(args, " "))
}
