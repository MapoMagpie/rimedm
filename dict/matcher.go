package dict

import (
	"context"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

type MatchResult struct {
	Entry  *Entry
	result algo.Result
}

func (m *MatchResult) String() string {
	return m.Entry.String()
}

func (m *MatchResult) Order() int {
	score := m.result.Score
	score = score * (200 * m.Entry.Weight) * (1000 - m.Entry.text.Length())
	return score
}

type Matcher interface {
	Search(key []rune, list []*Entry, resultChan chan<- []*MatchResult, ctx context.Context)
	Reset()
}

type CacheMatcher struct {
	cache map[string][]*MatchResult
}

func (m *CacheMatcher) Reset() {
	m.cache = nil
}

var slab = util.MakeSlab(100*1024, 2048)

func (m *CacheMatcher) Search(key []rune, list []*Entry, resultChan chan<- []*MatchResult, ctx context.Context) {
	var done bool
	go func() {
		<-ctx.Done()
		done = true
	}()
	var cache []*MatchResult
	if m.cache != nil {
		cachedKey := ""
		for i := len(key); i > 0; i-- {
			cachedKey = string(key[:i])
			if cache = m.cache[cachedKey]; cache != nil {
				break
			}
		}
		if done {
			return
		}
		if cache != nil && cachedKey == string(key) {
			resultChan <- cache
			return
		}
	}

	if cache != nil {
		list = make([]*Entry, len(cache))
		for i, m := range cache {
			list[i] = m.Entry
		}
	}

	matched := make([]*MatchResult, 0)
	lastIdx := 0
	listLen := len(list)
	chunkSize := 50000 // chunkSize = listLen means no async search
	for idx, entry := range list {
		if done {
			return
		}
		if entry.modType == DELETE {
			continue
		}
		result, _ := algo.FuzzyMatchV2(false, true, true, &entry.text, key, false, slab)
		if result.Score > 0 {
			matched = append(matched, &MatchResult{entry, result})
		}
		if (idx%chunkSize == 0 && idx != 0) || idx == listLen-1 {
			m2 := matched[lastIdx:]
			resultChan <- m2
			lastIdx = len(matched)
		}
	}

	if m.cache == nil {
		m.cache = make(map[string][]*MatchResult)
	}
	m.cache[string(key)] = matched
}
