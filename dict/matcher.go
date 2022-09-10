package dict

import (
	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
	"sort"
)

type entryResult struct {
	entry  *Entry
	result algo.Result
}

type Matcher interface {
	Search(key []rune, list []*Entry) []*Entry
	Reset()
}

type CacheMatcher struct {
	cache map[string][]*Entry
}

func (m *CacheMatcher) Reset() {
	m.cache = nil
}

var slab = util.MakeSlab(100*1024, 2048)

func (m *CacheMatcher) Search(key []rune, list []*Entry) []*Entry {
	if m.cache != nil {
		//m.cache = make(map[string][]*Entry)
		var cache []*Entry
		cachedKey := ""
		for i := len(key); i > 0; i-- {
			cachedKey = string(key[:i])
			if cache = m.cache[cachedKey]; cache != nil {
				list = cache
				break
			}
		}
		if cache != nil && cachedKey == string(key) {
			return cache
		}
	}
	matched := make([]entryResult, 0)
	for _, entry := range list {
		if entry.modType == DELETE {
			continue
		}
		result, _ := algo.FuzzyMatchV2(false, true, true, &entry.text, key, false, slab)
		if result.Score > 0 {
			matched = append(matched, entryResult{entry, result})
		}
	}
	if m.cache == nil {
		m.cache = make(map[string][]*Entry)
	}
	sort.Slice(matched, func(i, j int) bool {
		r := matched[i].result.Score - matched[j].result.Score
		if s := 1; r == 0 {
			if key[0] >= 0x80 {
				s = 0
			}
			r = len(matched[i].entry.Pair[s]) - len(matched[j].entry.Pair[s])
			if s = 1 - s; r == 0 {
				r = len(matched[i].entry.Pair[s]) - len(matched[j].entry.Pair[s])
			}
			return r < 0
		}
		return r > 0
	})
	entries := make([]*Entry, len(matched))
	for i, m := range matched {
		entries[i] = m.entry
	}
	m.cache[string(key)] = entries
	return entries
}
