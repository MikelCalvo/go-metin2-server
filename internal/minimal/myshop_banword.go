package minimal

import (
	"bytes"
	"sync"
)

const myShopOpenBanwordInfoMessage = "You can't give your shop an invalid name."

// bootstrapMyShopOpenBanwords is the deterministic in-process list for host-only
// MYSHOP open sign checks. Empty means the gate never fires. This slice is not
// a claim that DB BANWORDS reload or Canada-locale skip exist.
var bootstrapMyShopOpenBanwords = []string{
	"banme",
	"금지어",
}

var (
	myShopOpenBanwordMu       sync.Mutex
	myShopOpenBanwordOverride *[]string
)

// SetMyShopOpenBanwordsForTest replaces the bootstrap banword list for the
// duration of a test. Pass nil to restore the default bootstrap list. An empty
// non-nil slice proves the empty-list gate never rejects.
func SetMyShopOpenBanwordsForTest(words []string) func() {
	myShopOpenBanwordMu.Lock()
	defer myShopOpenBanwordMu.Unlock()
	previous := myShopOpenBanwordOverride
	if words == nil {
		myShopOpenBanwordOverride = nil
	} else {
		cloned := append([]string(nil), words...)
		myShopOpenBanwordOverride = &cloned
	}
	return func() {
		myShopOpenBanwordMu.Lock()
		defer myShopOpenBanwordMu.Unlock()
		myShopOpenBanwordOverride = previous
	}
}

func myShopOpenBanwordList() []string {
	myShopOpenBanwordMu.Lock()
	defer myShopOpenBanwordMu.Unlock()
	if myShopOpenBanwordOverride != nil {
		return append([]string(nil), (*myShopOpenBanwordOverride)...)
	}
	return append([]string(nil), bootstrapMyShopOpenBanwords...)
}

// myShopOpenSignContainsBanword reports whether sign contains any bootstrap
// banword as a case-sensitive contiguous byte substring. An empty list never
// matches. Matching walks the sign one byte at a time (UTF-8 / ASCII); this is
// behavior-equivalent to the oracle CheckString match for the English locale
// without inventing locale-specific two-byte steppers.
func myShopOpenSignContainsBanword(sign string) bool {
	words := myShopOpenBanwordList()
	if len(words) == 0 || sign == "" {
		return false
	}
	signBytes := []byte(sign)
	for _, word := range words {
		if word == "" {
			continue
		}
		if bytes.Contains(signBytes, []byte(word)) {
			return true
		}
	}
	return false
}
