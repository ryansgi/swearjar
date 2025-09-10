// Package domain defines the core types and interfaces for the ident service
package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

type (
	// HID32 is the fixed-size array form of a principal HID (map-key friendly)
	HID32 [32]byte

	// HID is the slice form of a principal HID (easy for DB/IO)
	HID []byte

	// Subject represents the type of principal
	Subject string
)

const (
	// SubjectRepo is a repository
	SubjectRepo Subject = "repo"

	// SubjectActor is a user or organization
	SubjectActor Subject = "actor"
)

// ResolverPort abstracts the operations needed to resolve principals and maps
type ResolverPort interface {
	RepoHID(ctx context.Context, resource string) (HID, bool, error) // owner/repo
	ActorHID(ctx context.Context, login string) (HID, bool, error)   // login
}

// UpserterPort abstracts the operations needed to ensure principals and maps exist
type UpserterPort interface {
	// map[[32]byte] -> github numeric id (repo_id/user_id)
	EnsurePrincipalsAndMaps(ctx context.Context,
		repos map[HID32]int64, actors map[HID32]int64,
	) error
}

// Repo abstracts the operations needed from a repository for backfilling
type Repo interface {
	// EnsurePrincipalsAndMaps ensures principals and mapping rows for the given HIDs.
	// The maps are from HID ([32]byte) to internal numeric ID (int64)
	// It is safe to call with empty maps (no-ops)
	// It is safe to call concurrently (with different maps)
	// It is recommended to limit concurrency to avoid DB overload
	EnsurePrincipalsAndMaps(ctx context.Context, repos map[HID32]int64, actors map[HID32]int64) error
}

// Ports is a convenience interface for ResolverPort and UpserterPort
type Ports interface {
	ResolverPort
	UpserterPort
}

// RepoHID32 computes the HID32 for a repo given its GitHub numeric ID
func RepoHID32(id int64) HID32 {
	return sha256.Sum256([]byte("repo:" + strconv.FormatInt(id, 10)))
}

// ActorHID32 computes the HID32 for an actor given its GitHub numeric ID
func ActorHID32(id int64) HID32 {
	return sha256.Sum256([]byte("actor:" + strconv.FormatInt(id, 10)))
}

// Bytes returns the slice form of the HID32
func (h HID32) Bytes() HID { return h[:] }

// Hex returns the lowercase hex encoding of the HID32
func (h HID32) Hex() string { return hex.EncodeToString(h[:]) }

// HID32FromBytes converts a byte slice to a HID32, returning ok=false if the length is wrong
func HID32FromBytes(b []byte) (HID32, bool) {
	var h HID32
	if len(b) != len(h) {
		return HID32{}, false
	}
	copy(h[:], b)
	return h, true
}

// HIDFromHex parses a 64-char hex string into HID ([]byte)
func HIDFromHex(s string) (HID, bool) {
	if len(s) != 64 {
		return nil, false
	}
	out := make([]byte, 32)
	n, err := hex.Decode(out, []byte(s))
	if err != nil || n != 32 {
		return nil, false
	}
	return HID(out), true
}
