package dict

import (
	"context"

	"github.com/sahilm/fuzzy"
)

type MatchResult struct {
	Entry *Entry
	score int
}

type MatchResultChunk struct {
	Result  []*MatchResult
	Version int
}

func (m *MatchResult) Id() int {
	return int(m.Entry.FID)
}

func (m *MatchResult) String() string {
	return string(m.Entry.raw)
}

func (m *MatchResult) Cmp(other any) bool {
	if o, ok := other.(*MatchResult); ok {
		if m.score == o.score {
			return m.Entry.data.Weight > o.Entry.data.Weight
		}
		return m.score > o.score
	}
	return false
}

type Matcher interface {
	Search(key string, useColumn Column, searchVersion int, list []*Entry, resultChan chan<- MatchResultChunk, ctx context.Context)
	Reset()
}

type CacheMatcher struct {
	cache map[string][]*MatchResult
}

func (m *CacheMatcher) Reset() {
	m.cache = nil
}

// var slab = util.MakeSlab(200*1024, 4096)

func (m *CacheMatcher) Search(key string, useColumn Column, searchVersion int, list []*Entry, resultChan chan<- MatchResultChunk, ctx context.Context) {
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
			resultChan <- MatchResultChunk{Result: cache, Version: searchVersion}
			return
		}
	}

	if cache != nil {
		list = make([]*Entry, len(cache))
		for i, m := range cache {
			list[i] = m.Entry
		}
	}

	getTarget := func(entry *Entry) string {
		return entry.data.Code
	}
	if useColumn != COLUMN_CODE {
		if useColumn == COLUMN_TEXT {
			getTarget = func(entry *Entry) string {
				return entry.data.Text
			}
		} else {
			getTarget = func(entry *Entry) string {
				return entry.raw
			}
		}
	}

	matched := make([]*MatchResult, 0)
	listLen := len(list)
	chunkSize := 50000 // chunkSize = listLen means no async search
	for c := 0; c < listLen; c += chunkSize {
		end := c + chunkSize
		if end > listLen {
			end = listLen
		}
		chunk := list[c:end]
		source := &ChunkSource{chunk, getTarget}
		matches := fuzzy.FindFromNoSort(key, source)
		if len(matches) == 0 {
			continue
		}
		ret := make([]*MatchResult, 0, len(matches))
		for _, ma := range matches {
			if chunk[ma.Index].IsDelete() { // cache matcher still need to determine whether it has been deleted
				continue
			}
			ret = append(ret, &MatchResult{chunk[ma.Index], ma.Score})
		}
		if len(ret) > 0 {
			resultChan <- MatchResultChunk{Result: ret, Version: searchVersion}
			matched = append(matched, ret...)
		}
	}
	// log.Printf("Cache Matcher Search: Key: %s, List Len: %d, Cached: %v, Matched: %d", string(key), listLen, cache != nil, len(matched))
	if m.cache == nil {
		m.cache = make(map[string][]*MatchResult)
	}
	m.cache[string(key)] = matched
}

type ChunkSource struct {
	chunk     []*Entry
	getTarget func(e *Entry) string
}

func (e *ChunkSource) Len() int {
	return len(e.chunk)
}

func (e *ChunkSource) String(i int) string {
	return e.getTarget(e.chunk[i])
}
