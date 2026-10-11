package internal

import (
	"reflect"
	"testing"
)

// Ruby operator methods are methods too: their names are not identifiers, so the
// name pattern has to allow them. The span of each runs to its own end.
func TestDeclarations_RubyOperatorMethods(t *testing.T) {
	src := `class Money
  def <=> other
    value <=> other.value
  end

  def [](key)
    @data[key]
  end

  def []=(key, value)
    @data[key] = value
  end

  def ==(other)
    other.is_a?(Money) && value == other.value
  end

  def -@
    Money.new(-value)
  end
end
`
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"-@:18-20", "<=>:2-4", "==:14-16", "[]:6-8", "[]=:10-12"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A slash after a regex-introducing word or a bracket starts a regular expression, whose
// quote is not a string. A slash after an operand is a division.
func TestDeclarations_RubyRegexLiteralHoldsQuotes(t *testing.T) {
	src := `def kind(s)
  case s
  when /'/ then :quoted
  end
end

def split_words(text)
  text.gsub(/[^\-a-z']+/) { |s| s }
end

def half(x)
  x / 2 + x / 3
end
`
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"half:11-13", "kind:1-5", "split_words:7-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A percent literal runs to its closing bracket, nested brackets included. Quotes and
// slashes inside it are text: the method after it is still found.
func TestDeclarations_RubyPercentLiteralsHoldNoCode(t *testing.T) {
	src := `def first
  x = %q{hello world}
  y = %r{/?\z}
  x
end

def second
  y = %Q{a {nested} b}
  y
end

def third(s)
  s.gsub(%w[a b], "")
end
`
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"first:1-5", "second:7-10", "third:12-14"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// public def and private def are method definitions with a visibility in front.
func TestDeclarations_RubyVisibilityPrefixedDef(t *testing.T) {
	src := `class Event
  public def to_liquid
    Drop.new(self)
  end

  private def perform(request)
    request
  end
end
`
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"perform:6-8", "to_liquid:2-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}
