package internal

// The pilot is test-only until C#, Java and D have independent corpus results.
// Language adapters provide events; the walker has no language/token rules.
type braceContext struct {
	className  string
	inFunction bool
}

type braceEvent struct {
	boundary bool
	end      int
	body     int
	next     int
	reset    bool
	class    *ClassBounds
	function *FunctionBounds
}

type braceAdapter interface {
	Len() int
	Event(index, boundary int, context braceContext) braceEvent
}

func (f *Finder) findBraceRecursive(lines []string, startLine int, filename string, adapter braceAdapter) (*FindResult, error) {
	result := &FindResult{Filename: filename}
	var walk func(int, int, braceContext)
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
					result.Functions = append(result.Functions, fn)
				}
				if event.body >= 0 && event.end < limit {
					child := context
					child.inFunction = true
					walk(event.body+1, event.end, child)
					i = event.end
				} else {
					i = event.next
				}
				boundary = i + 1
			} else if event.end > i && event.end < limit {
				child := context
				if event.class != nil {
					result.Classes = append(result.Classes, *event.class)
					child = braceContext{className: event.class.Name}
				}
				walk(i+1, event.end, child)
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
	walk(0, adapter.Len(), braceContext{})
	return result, nil
}
