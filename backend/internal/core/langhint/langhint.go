// Package langhint provides language and script detection utilities.
package langhint

import (
	"unicode"
)

// DetectScriptAndLang returns a coarse script name (always) and a best-effort BCP-47 lang code
// Lang is only set if letterCount >= minLetters and the script -> language mapping is strong.
func DetectScriptAndLang(s string) (script string, lang string) {
	const minLetters = 20

	// Counters by script
	var (
		latin, cyrillic, greek, han, hira, kata, hangul int
		arabic, hebrew, thai, georgian, armenian        int
		devanagari                                      int
		totalLetters                                    int
	)

	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		totalLetters++

		switch {
		case unicode.In(r, unicode.Hangul):
			hangul++
		case unicode.In(r, unicode.Hiragana):
			hira++
		case unicode.In(r, unicode.Katakana):
			kata++
		case unicode.In(r, unicode.Han):
			han++
		case unicode.In(r, unicode.Arabic):
			arabic++
		case unicode.In(r, unicode.Hebrew):
			hebrew++
		case unicode.In(r, unicode.Thai):
			thai++
		case unicode.In(r, unicode.Greek):
			greek++
		case unicode.In(r, unicode.Cyrillic):
			cyrillic++
		case unicode.In(r, unicode.Georgian):
			georgian++
		case unicode.In(r, unicode.Armenian):
			armenian++
		case unicode.In(r, unicode.Devanagari):
			devanagari++
		default:
			if unicode.In(r, unicode.Latin) {
				latin++
			}
		}
	}

	// Choose predominant script; tie-break prefers specific scripts over Latin
	type sc struct {
		name string
		cnt  int
	}
	cands := []sc{
		{"Hiragana", hira},
		{"Katakana", kata},
		{"Hangul", hangul},
		{"Han", han},
		{"Arabic", arabic},
		{"Hebrew", hebrew},
		{"Thai", thai},
		{"Greek", greek},
		{"Cyrillic", cyrillic},
		{"Georgian", georgian},
		{"Armenian", armenian},
		{"Devanagari", devanagari},
		{"Latin", latin},
	}
	var best sc
	for _, c := range cands {
		if c.cnt > best.cnt {
			best = c
		}
	}
	script = best.name
	if best.cnt == 0 {
		// Fallback when there were no letters at all
		script = ""
	}

	// Only emit lang when we have enough letters and the mapping is strong/low-ambiguity.
	if totalLetters >= minLetters {
		switch {
		// Japanese: presence of Hiragana or Katakana is decisive
		case hira > 0 || kata > 0:
			lang = "ja"
		// Korean: Hangul is decisive
		case hangul > 0:
			lang = "ko"
		// Arabic/Hebrew/Thai/Greek are typically unambiguous in practice
		case arabic > 0:
			lang = "ar"
		case hebrew > 0:
			lang = "he"
		case thai > 0:
			lang = "th"
		case greek > 0:
			lang = "el"
		// Scripts with higher ambiguity - leave unset (NULL):
		// Han (zh/ja mixed), Cyrillic (ru/uk/bg/...), Devanagari (hi/mr/ne/...), etc.
		default:
			lang = ""
		}
	}

	return script, lang
}
