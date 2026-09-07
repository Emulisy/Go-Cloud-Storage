package files

// Metadata describes a stored file without containing its bytes.
//
// This is a domain type. It intentionally has no JSON or database tags because
// the HTTP and database layers will define their own representations.
type Metadata struct {
	ID       string
	Name     string
	Size     int64
	Checksum string
}
