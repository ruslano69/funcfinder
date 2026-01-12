package internal

// State представляет текущее состояние парсера
type State int

const (
	StateNormal State = iota
	StateLineComment
	StateBlockComment
	StateString
	StateRawString
)

// Sanitizer очищает строки от комментариев и литералов
type Sanitizer struct {
	config *LanguageConfig
	useRaw bool
}

// NewSanitizer создает новый санитайзер
func NewSanitizer(config *LanguageConfig, useRaw bool) *Sanitizer {
	return &Sanitizer{
		config: config,
		useRaw: useRaw,
	}
}

// CleanLine очищает строку от комментариев и литералов
// Возвращает очищенную строку и новое состояние
func (s *Sanitizer) CleanLine(line string, state State) (string, State) {
	if len(line) == 0 {
		return line, state
	}
	
	result := make([]rune, len(line))
	for i := range result {
		result[i] = ' '
	}
	
	runes := []rune(line)
	i := 0
	
	for i < len(runes) {
		switch state {
		case StateBlockComment:
			// Ищем конец блочного комментария
			if s.matchesAt(runes, i, s.config.BlockCommentEnd) {
				i += len([]rune(s.config.BlockCommentEnd))
				state = StateNormal
			} else {
				i++
			}
			
		case StateString:
			// Внутри строкового литерала
			if runes[i] == []rune(s.config.EscapeChar)[0] && i+1 < len(runes) {
				// Экранированный символ
				i += 2
			} else if s.matchesAnyAt(runes, i, s.config.StringChars) {
				// Конец строки
				i++
				state = StateNormal
			} else {
				i++
			}
			
		case StateRawString:
			// Внутри raw-строки (без экранирования)
			if s.matchesAnyAt(runes, i, s.config.RawStringChars) {
				i++
				state = StateNormal
			} else {
				if s.useRaw {
					result[i] = runes[i]
				}
				i++
			}
			
		case StateNormal:
			// Проверяем начало линейного комментария
			if s.matchesAt(runes, i, s.config.LineComment) {
				// Линейный комментарий до конца строки
				// Остаток строки пропускаем, state остается Normal
				return string(result), StateNormal
			}
			
			// Проверяем начало блочного комментария
			if s.matchesAt(runes, i, s.config.BlockCommentStart) {
				state = StateBlockComment
				i += len([]rune(s.config.BlockCommentStart))
				continue
			}
			
			// Проверяем начало raw-строки
			if !s.useRaw && s.matchesAnyAt(runes, i, s.config.RawStringChars) {
				state = StateRawString
				i++
				continue
			}
			
			// Проверяем начало обычной строки
			if s.matchesAnyAt(runes, i, s.config.StringChars) {
				state = StateString
				i++
				continue
			}
			
			// Обычный символ кода
			result[i] = runes[i]
			i++
		}
	}
	
	return string(result), state
}

// matchesAt проверяет совпадение строки в указанной позиции
func (s *Sanitizer) matchesAt(runes []rune, pos int, pattern string) bool {
	patternRunes := []rune(pattern)
	if pos+len(patternRunes) > len(runes) {
		return false
	}
	for i, pr := range patternRunes {
		if runes[pos+i] != pr {
			return false
		}
	}
	return true
}

// matchesAnyAt проверяет совпадение любой из строк в указанной позиции
func (s *Sanitizer) matchesAnyAt(runes []rune, pos int, patterns []string) bool {
	for _, pattern := range patterns {
		if s.matchesAt(runes, pos, pattern) {
			return true
		}
	}
	return false
}

// CountBraces подсчитывает баланс фигурных скобок в очищенной строке
func CountBraces(line string) int {
	count := 0
	for _, ch := range line {
		if ch == '{' {
			count++
		} else if ch == '}' {
			count--
		}
	}
	return count
}
