package repo

import (
	"context"
	"sort"
	"strings"
	"time"

	"swearjar/internal/services/api/swearjar/domain"
)

func (s *hybridStore) CategoriesStack(
	ctx context.Context,
	in domain.CategoriesStackInput,
) (domain.CategoriesStackResp, error) {
	sevOrder := []string{"mild", "strong", "slur_masked"}
	if len(in.Severities) > 0 {
		// Keep only known in provided order
		allowed := map[string]bool{"mild": true, "strong": true, "slur_masked": true}
		tmp := make([]string, 0, len(in.Severities))
		for _, s := range in.Severities {
			if allowed[s] {
				tmp = append(tmp, s)
			}
		}
		if len(tmp) > 0 {
			sevOrder = tmp
		}
	}

	// Sorting key
	sortBy := strings.TrimSpace(in.SortBy)
	if sortBy == "" {
		sortBy = "hits"
	}
	isSevSort := sortBy == "mild" || sortBy == "strong" || sortBy == "slur_masked"

	// TopN, IncludeOther, AsShare
	topN := in.TopN
	if topN <= 0 {
		topN = 8
	}
	includeOther := false
	if in.IncludeOther != nil {
		includeOther = *in.IncludeOther
	}
	asShare := false
	if in.AsShare != nil {
		asShare = *in.AsShare
	}

	// Time window [start, end] inclusive days (convert to [start, end+1d) Datetime)
	startDay, err := time.Parse("2006-01-02", in.Range.Start)
	if err != nil {
		return domain.CategoriesStackResp{}, err
	}
	endDay, err := time.Parse("2006-01-02", in.Range.End)
	if err != nil {
		return domain.CategoriesStackResp{}, err
	}
	startTS := time.Date(startDay.Year(), startDay.Month(), startDay.Day(), 0, 0, 0, 0, time.UTC)
	endTS := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)

	where := []string{
		"created_at >= ?",
		"created_at < ?",
	}
	args := []any{startTS, endTS}

	if len(in.DetVer) > 0 {
		where = append(where, "detver IN ?")
		args = append(args, in.DetVer)
	}
	if len(in.RepoHIDs) > 0 {
		where = append(where, "repo_hid IN ?")
		args = append(args, in.RepoHIDs)
	}
	if len(in.ActorHIDs) > 0 {
		where = append(where, "actor_hid IN ?")
		args = append(args, in.ActorHIDs)
	}
	if len(in.NLLangs) > 0 {
		where = append(where, "lang_code IN ?")
		args = append(args, in.NLLangs)
	}
	if in.LangReliable != nil {
		if *in.LangReliable {
			where = append(where, "lang_reliable = 1")
		} else {
			where = append(where, "lang_reliable = 0")
		}
	}

	// Severities filter (if provided)
	if len(in.Severities) > 0 {
		where = append(where, "severity IN ?")
		args = append(args, in.Severities)
	}
	// Categories filter (if provided)
	if len(in.Categories) > 0 {
		where = append(where, "category IN ?")
		args = append(args, in.Categories)
	}

	// Map NULL/empty to a stable placeholder so grouping is predictable.
	// We'll keep raw label for display; unknowns will get "unknown"
	sql := `
		SELECT
		  cast(category AS Nullable(String))                        AS raw_cat,
		  ifNull(nullIf(category, ''), '__unknown__')               AS cat,
		  severity                                                  AS sev,
		  count()                                                   AS hits
		FROM swearjar.commit_crimes
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY raw_cat, cat, sev
		ORDER BY cat ASC, sev ASC
	`

	type row struct {
		RawCat *string
		Cat    string
		Sev    string
		Hits   uint64
	}

	rs, err := s.ch.Query(ctx, sql, args...)
	if err != nil {
		return domain.CategoriesStackResp{}, err
	}
	defer rs.Close()

	// Accumulators
	type acc struct {
		label string
		bySev map[string]int64
		total int64
	}
	byCat := map[string]*acc{}
	totalsBySev := map[string]int64{}
	var grandTotal int64

	for rs.Next() {
		var r row
		if err := rs.Scan(&r.RawCat, &r.Cat, &r.Sev, &r.Hits); err != nil {
			return domain.CategoriesStackResp{}, err
		}
		a := byCat[r.Cat]
		if a == nil {
			label := "unknown"
			if r.RawCat != nil && strings.TrimSpace(*r.RawCat) != "" {
				label = *r.RawCat
			}
			a = &acc{
				label: label,
				bySev: make(map[string]int64, 4),
			}
			byCat[r.Cat] = a
		}
		h := int64(r.Hits)
		a.bySev[r.Sev] += h
		a.total += h
		totalsBySev[r.Sev] += h
		grandTotal += h
	}
	if err := rs.Err(); err != nil {
		return domain.CategoriesStackResp{}, err
	}

	// If no data, return empty shell with echoed window
	if len(byCat) == 0 {
		return domain.CategoriesStackResp{
			SeverityKeys:   sevOrder,
			TotalsBySev:    map[string]int64{},
			TotalsShareSev: map[string]float64{},
			SortedBy:       sortBy,
			Window:         in.Range,
			Stack:          []domain.CategoryStackItem{},
			TotalHits:      0,
		}, nil
	}

	// Build sortable slice
	type ranked struct {
		key   string
		label string
		bySev map[string]int64
		total int64
		share float64 // total / grandTotal (0..1); used when sort_by = "share"
	}
	items := make([]ranked, 0, len(byCat))
	for k, a := range byCat {
		it := ranked{
			key:   k,
			label: a.label,
			bySev: a.bySev,
			total: a.total,
		}
		if grandTotal > 0 {
			it.share = float64(a.total) / float64(grandTotal)
		}
		items = append(items, it)
	}

	// Sort
	sort.Slice(items, func(i, j int) bool {
		// Primary: chosen metric desc
		var vi, vj int64
		switch {
		case sortBy == "hits":
			vi, vj = items[i].total, items[j].total
		case sortBy == "share":
			// compare by float, but keep tie-breakers deterministic
			if items[i].share == items[j].share {
				break
			}
			return items[i].share > items[j].share
		case isSevSort:
			vi, vj = items[i].bySev[sortBy], items[j].bySev[sortBy]
		default:
			vi, vj = items[i].total, items[j].total
		}
		if vi != vj {
			return vi > vj
		}
		// Tie: lexicographic by label for stability
		if items[i].label != items[j].label {
			return items[i].label < items[j].label
		}
		return items[i].key < items[j].key
	})

	// Apply TopN
	head := items
	tail := []ranked{}
	if len(items) > topN {
		head = items[:topN]
		tail = items[topN:]
	}

	// Optionally fold remainder into "__other__"
	if includeOther && len(tail) > 0 {
		other := ranked{
			key:   "__other__",
			label: "other",
			bySev: map[string]int64{},
			total: 0,
			share: 0,
		}
		for _, t := range tail {
			other.total += t.total
			for _, sev := range sevOrder {
				other.bySev[sev] += t.bySev[sev]
			}
		}
		if grandTotal > 0 {
			other.share = float64(other.total) / float64(grandTotal)
		}
		head = append(head, other)
	}

	stack := make([]domain.CategoryStackItem, 0, len(head))
	for _, it := range head {
		item := domain.CategoryStackItem{
			Key:        it.key,
			Label:      it.label,
			Counts:     map[string]int64{},
			Mild:       it.bySev["mild"],
			Strong:     it.bySev["strong"],
			SlurMasked: it.bySev["slur_masked"],
			Total:      it.total,
		}

		for _, sev := range sevOrder {
			item.Counts[sev] = it.bySev[sev]
		}

		if asShare && grandTotal > 0 {
			item.Shares = map[string]float64{}
			for _, sev := range sevOrder {
				item.Shares[sev] = float64(it.bySev[sev]) / float64(grandTotal)
			}
		}
		stack = append(stack, item)
	}

	// Totals share by severity (0..1) - relative to grand total
	totalsShare := map[string]float64{}
	if asShare && grandTotal > 0 {
		for _, sev := range sevOrder {
			totalsShare[sev] = float64(totalsBySev[sev]) / float64(grandTotal)
		}
	}

	out := domain.CategoriesStackResp{
		SeverityKeys:   sevOrder,
		TotalsBySev:    totalsBySev,
		TotalsShareSev: totalsShare, // present iff asShare=true and data>0 (could be empty map otherwise)
		SortedBy:       sortBy,
		Window:         in.Range,
		Stack:          stack,
		TotalHits:      grandTotal,
	}
	// If !asShare, zero out TotalsShareSev to avoid confusing clients
	if !asShare {
		out.TotalsShareSev = nil
	}
	return out, nil
}
