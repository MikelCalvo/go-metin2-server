package loginticket

// AuthLoginTicketHandoffExporter is the first repository-style seam for the
// durable authd-to-gamed login-ticket handoff already projected onto migration
// 0007_auth_login_ticket_handoff. Implementations may be file-backed or hermetic
// in-memory; none of these methods open a database, emit SQL, consume tickets,
// or mutate stores beyond the Issue/Load/Consume path already owned by Store.
//
// Missing or empty pending-ticket sets are treated as an empty migration-shaped
// export.
type AuthLoginTicketHandoffExporter interface {
	ExportAuthLoginTicketHandoff() (AuthLoginTicketHandoffExport, error)
}

var (
	_ Store                          = (*FileStore)(nil)
	_ AuthLoginTicketHandoffExporter = (*FileStore)(nil)
	_ Store                          = (*MemoryStore)(nil)
	_ AuthLoginTicketHandoffExporter = (*MemoryStore)(nil)
)
