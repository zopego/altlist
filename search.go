package altlist

import (
	"strings"

	"slices"

	"github.com/charmbracelet/bubbles/list"
	"github.com/sahilm/fuzzy"
)

type SearchOption struct {
	CaseSensitive    bool
	MatchesOnly      bool // if true, only items with matches are returned
	SortByMatchCount bool // if true, ranks are sorted by match count
	ReverseSort      bool // if true, ranks are sorted in descending order
}

// DefaultFilter uses the sahilm/fuzzy to filter through the list.
// This is set by default.
func AltFilter(term string, targets []string) []list.Rank {
	itemMap := make(map[int]bool)
	ranks := fuzzy.Find(term, targets)
	//sort.Stable(ranks)
	result := make([]list.Rank, len(targets))
	for i, r := range ranks {
		k := r.MatchedIndexes
		for j, index := range k {
			k[j] = index
		}
		result[i] = list.Rank{
			Index:          r.Index,
			MatchedIndexes: k,
		}
		itemMap[r.Index] = true
	}
	// find all remaining items
	j := 0
	for i := range targets {
		if _, ok := itemMap[i]; !ok {
			result[j+len(ranks)] = list.Rank{Index: i, MatchedIndexes: []int{}}
			j++
		}
	}
	return result
}

func MakeSearchFunc(option SearchOption) func(term string, targets []string) []list.Rank {
	return func(term string, targets []string) []list.Rank {
		if !option.CaseSensitive {
			term = strings.ToLower(term)
		}
		terms := strings.Split(term, " ")

		ranks := []list.Rank{}
		for idx, r := range targets {
			rank := list.Rank{Index: idx}
			for _, t := range terms {
				// find indices of matches of t in r
				match := strings.Index(r, t)
				if match != -1 {
					newMatchIndixes := make([]int, len(t))
					for i, _ := range newMatchIndixes {
						newMatchIndixes[i] = match + i
					}
					//append to rank
					rank.MatchedIndexes = append(rank.MatchedIndexes, newMatchIndixes...)
				}
			}
			if len(rank.MatchedIndexes) > 0 || !option.MatchesOnly {
				ranks = append(ranks, rank)
			}
		}

		if option.SortByMatchCount {
			slices.SortStableFunc(ranks, func(i, j list.Rank) int {
				if option.ReverseSort {
					return len(j.MatchedIndexes) - len(i.MatchedIndexes)
				}
				return len(i.MatchedIndexes) - len(j.MatchedIndexes)
			})
		}

		return ranks
	}
}
