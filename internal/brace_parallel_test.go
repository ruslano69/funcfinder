package internal

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

// Shared by every file in a CLI process, preventing file workers from each
// starting a separate CPU-sized pool. A busy parent falls back to inline work.
type braceScopeExecutor struct {
	slots     chan struct{}
	minTokens int
	started   atomic.Int64
}

func newBraceScopeExecutor(workers, minTokens int) *braceScopeExecutor {
	return &braceScopeExecutor{slots: make(chan struct{}, max(0, workers-1)), minTokens: minTokens}
}

func braceScopeSetting(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

var braceParallelExecutor = newBraceScopeExecutor(
	min(runtime.NumCPU(), braceScopeSetting("FUNCFINDER_SCOPE_WORKERS", runtime.NumCPU())),
	braceScopeSetting("FUNCFINDER_SCOPE_MIN_TOKENS", 2048),
)

func (f *Finder) findBraceFunctionsParallel(lines []string, startLine int, filename string) (*FindResult, error) {
	return f.findBraceFunctionsParallelWith(lines, startLine, filename, braceParallelExecutor)
}

func (f *Finder) findBraceFunctionsParallelWith(lines []string, startLine int, filename string, executor *braceScopeExecutor) (*FindResult, error) {
	// Keep preparation identical to the serial pilot to isolate scope scheduling.
	a := &braceTokenAdapter{config: f.config, startLine: startLine}
	var facts []LineFacts
	if f.config.LangKey == "d" {
		prepared, ends := prepareDLines(lines, f.config)
		a.headerEnds = ends
		facts = lexerFacts(prepared, lines)
	} else {
		facts = f.sanitizer.CleanLinesFacts(lines)
	}
	a.tokens, a.pairs = csTokens(facts)
	a.conditionals = csConditionals(facts)
	return f.findBraceParallel(lines, startLine, filename, a, executor)
}

func (f *Finder) findBraceParallel(lines []string, startLine int, filename string, adapter braceAdapter, executor *braceScopeExecutor) (*FindResult, error) {
	var scan func(int, int, braceContext) *FindResult
	scan = func(first, limit int, context braceContext) *FindResult {
		// Each task owns its slices. A placeholder records a child task's exact
		// source position; join in that order after all children finish.
		current := &FindResult{}
		parts := []*FindResult{current}
		var pending sync.WaitGroup
		var walk func(int, int, braceContext)
		var descend func(int, int, braceContext)
		descend = func(first, limit int, context braceContext) {
			if limit-first >= executor.minTokens {
				select {
				case executor.slots <- struct{}{}:
					child := &FindResult{}
					current = &FindResult{}
					parts = append(parts, child, current)
					pending.Add(1)
					executor.started.Add(1)
					go func() {
						defer pending.Done()
						defer func() { <-executor.slots }()
						*child = *scan(first, limit, context)
					}()
					return
				default:
				}
			}
			walk(first, limit, context)
		}
		walk = func(first, limit int, context braceContext) {
			boundary := first
			for i := first; i < limit; i++ {
				event := adapter.Event(i, boundary, context)
				if event.function != nil {
					fn := *event.function
					if f.extractMode {
						fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
					}
					if f.mapMode || f.funcNames[fn.Name] {
						current.Functions = append(current.Functions, fn)
					}
					if event.body >= 0 && event.end < limit {
						child := context
						child.inFunction = true
						descend(event.body+1, event.end, child)
						i = event.end
					} else {
						i = event.next
					}
					boundary = i + 1
				} else if event.end > i && event.end < limit {
					child := context
					if event.class != nil {
						current.Classes = append(current.Classes, *event.class)
						child = braceContext{className: event.class.Name}
					}
					descend(i+1, event.end, child)
					if event.reset {
						boundary = event.end + 1
					}
					i = event.end
				} else if event.boundary {
					boundary = i + 1
				} else if event.next > i {
					i = event.next
				}
			}
		}
		walk(first, limit, context)
		pending.Wait()
		if len(parts) == 1 {
			return current
		}
		joined := &FindResult{}
		for _, part := range parts {
			joined.Functions = append(joined.Functions, part.Functions...)
			joined.Classes = append(joined.Classes, part.Classes...)
		}
		return joined
	}
	result := scan(0, adapter.Len(), braceContext{})
	result.Filename = filename
	return result, nil
}
