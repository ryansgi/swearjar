package gharchive

import (
	json "encoding/json/v2"
	"fmt"
	"strings"
	"time"
)

// GHTime is an alias so it can be used anywhere a time.Time is expected
type GHTime = time.Time

// RawJSON captures raw JSON bytes without depending on json.RawMessage/RawValue
type RawJSON []byte

// UnmarshalJSON implements json.Unmarshaler by copying bytes
func (r *RawJSON) UnmarshalJSON(b []byte) error {
	*r = append((*r)[:0], b...)
	return nil
}

// Boolish decodes true/false, 0/1, and "true"/"false"/"0"/"1"
type Boolish bool

// UnmarshalJSON implements json.Unmarshaler for Boolish
func (b *Boolish) UnmarshalJSON(data []byte) error {
	var vb bool
	if err := json.Unmarshal(data, &vb); err == nil {
		*b = Boolish(vb)
		return nil
	}
	var vi int64
	if err := json.Unmarshal(data, &vi); err == nil {
		*b = Boolish(vi != 0)
		return nil
	}
	var vs string
	if err := json.Unmarshal(data, &vs); err == nil {
		switch strings.ToLower(strings.TrimSpace(vs)) {
		case "true", "1":
			*b = true
		default:
			*b = false
		}
		return nil
	}
	*b = false
	return nil
}

// Actor is the "actor" field in GHArchive events
type Actor struct {
	Login string `json:"login,omitempty"`
	ID    int64  `json:"id,omitempty"`
}

// Repo is the "repo" field in GHArchive events
type Repo struct {
	Name string `json:"name,omitempty"` // "owner/name" when derivable
	ID   int64  `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
}

// EventEnvelope is one GHArchive line as an envelope
type EventEnvelope struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Public    Boolish `json:"public"`
	Actor     Actor   `json:"actor"`
	Repo      Repo    `json:"repo"`
	CreatedAt GHTime  `json:"created_at"`

	// raw payload bytes for late binding
	Payload    []byte `json:"payload"`
	RawPayload []byte `json:"-"`
}

// UnmarshalJSON handles 2012-era schema drift (actor/repo/public/timestamps/payload)
func (e *EventEnvelope) UnmarshalJSON(data []byte) error {
	var raw map[string]RawJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	_ = json.Unmarshal(raw["id"], &e.ID)
	_ = json.Unmarshal(raw["type"], &e.Type)
	_ = json.Unmarshal(raw["public"], &e.Public)

	// created_at: RFC3339 or "2006/01/02 15:04:05 -0700"
	if v := raw["created_at"]; len(v) > 0 {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				e.CreatedAt = t
			} else if t, err := time.Parse("2006/01/02 15:04:05 -0700", s); err == nil {
				e.CreatedAt = t
			}
		}
	}

	// payload -> raw bytes
	if v := raw["payload"]; len(v) > 0 {
		e.Payload = append(e.Payload[:0], v...)
	}
	e.RawPayload = data

	var actorLogin string
	var actorID int64

	if v := raw["actor"]; len(v) > 0 {
		var as string
		if err := json.Unmarshal(v, &as); err == nil && as != "" {
			actorLogin = as
		} else {
			var ao struct {
				Login string `json:"login"`
				ID    int64  `json:"id"`
			}
			if err := json.Unmarshal(v, &ao); err == nil {
				actorLogin, actorID = ao.Login, ao.ID
			}
		}
	}
	if actorLogin == "" && len(raw["actor_attributes"]) > 0 {
		var aa struct {
			Login string `json:"login"`
		}
		_ = json.Unmarshal(raw["actor_attributes"], &aa)
		actorLogin = aa.Login
	}
	e.Actor = Actor{Login: actorLogin, ID: actorID}

	var rName, rURL string
	var rID int64

	if v := raw["repo"]; len(v) > 0 {
		var ro struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			URL  string `json:"url"`
		}
		if err := json.Unmarshal(v, &ro); err == nil {
			rID, rName, rURL = ro.ID, ro.Name, ro.URL
		}
	}
	if rName == "" && len(raw["repository"]) > 0 {
		var legacy map[string]RawJSON
		if err := json.Unmarshal(raw["repository"], &legacy); err == nil {
			var name string
			_ = json.Unmarshal(legacy["name"], &name)

			var owner string
			if o := legacy["owner"]; len(o) > 0 {
				// string owner or object with login
				if err := json.Unmarshal(o, &owner); err != nil {
					var obj struct {
						Login string `json:"login"`
					}
					if json.Unmarshal(o, &obj) == nil {
						owner = obj.Login
					}
				}
			}
			if owner != "" && name != "" {
				rName = owner + "/" + name
			} else if name != "" {
				rName = name
			}
			_ = json.Unmarshal(legacy["url"], &rURL)
			_ = json.Unmarshal(legacy["id"], &rID)
		}
	}
	e.Repo = Repo{Name: rName, ID: rID, URL: rURL}
	return nil
}

// HourRef identifies an archive hour by fields (not methods) so existing uses compile
type HourRef struct {
	Year  int
	Month int
	Day   int
	Hour  int
}

// NewHourRef constructs an HourRef from a time.Time (in any timezone)
func NewHourRef(t time.Time) HourRef {
	return HourRef{
		Year:  t.Year(),
		Month: int(t.Month()),
		Day:   t.Day(),
		Hour:  t.Hour(),
	}
}

// String formats as "YYYY-MM-DD-HH"
func (h HourRef) String() string {
	return fmt.Sprintf("%04d-%02d-%02d-%d", h.Year, h.Month, h.Day, h.Hour)
}

// Before reports whether the hour is before the given time
func (h HourRef) Before(t time.Time) bool {
	ht := time.Date(h.Year, time.Month(h.Month), h.Day, h.Hour, 0, 0, 0, time.UTC)
	return ht.Before(t)
}
