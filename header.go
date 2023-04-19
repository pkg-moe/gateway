package gateway

import (
	"strings"
)

func HeaderMatcher(key string) (string, bool) {
	return strings.ToLower(key), true
}
