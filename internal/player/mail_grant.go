package player

import (
	"strconv"
	"strings"

	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
)

// GrantCarriedMailItem places one mail or letter grant into carried inventory
// using the already-owned stack placement contract. It does not debit gold and
// does not invent mailbox UI, mail attach/detach, or mall checkout.
func (r *Runtime) GrantCarriedMailItem(template itemcatalog.Template, count uint16) (CarriedItemGrantResult, bool) {
	return r.GrantCarriedItem(template, count)
}

// ParseMailGrantCommand recognizes the first lab mail/letter grant slash.
// Valid forms are `/mail_grant <vnum>` and `/mail_grant <vnum> <count>`, plus
// the `/letter_grant` alias. Count defaults to 1. Recognized but malformed
// attempts stay recognized so the chat handler can consume them fail-closed
// instead of falling through as ordinary talking chat.
func ParseMailGrantCommand(message string) (vnum uint32, count uint16, parsed bool, recognized bool) {
	if !strings.HasPrefix(message, "/") {
		return 0, 0, false, false
	}
	fields := strings.Fields(strings.TrimSpace(message[1:]))
	if len(fields) == 0 || (fields[0] != "mail_grant" && fields[0] != "letter_grant") {
		return 0, 0, false, false
	}
	switch len(fields) {
	case 2:
		parsedVnum, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil || parsedVnum == 0 {
			return 0, 0, false, true
		}
		return uint32(parsedVnum), 1, true, true
	case 3:
		parsedVnum, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil || parsedVnum == 0 {
			return 0, 0, false, true
		}
		parsedCount, err := strconv.ParseUint(fields[2], 10, 16)
		if err != nil || parsedCount == 0 {
			return 0, 0, false, true
		}
		return uint32(parsedVnum), uint16(parsedCount), true, true
	default:
		return 0, 0, false, true
	}
}
