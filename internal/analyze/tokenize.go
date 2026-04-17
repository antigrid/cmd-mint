package analyze

import "unicode"

func Tokenize(command string) []string {
	var tokens []string
	var current []rune
	var quote rune
	var escaped bool
	inToken := false

	flush := func() {
		if inToken {
			tokens = append(tokens, string(current))
			current = current[:0]
			inToken = false
		}
	}

	for _, r := range command {
		switch {
		case escaped:
			current = append(current, r)
			inToken = true
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
			inToken = true
		case quote == 0 && unicode.IsSpace(r):
			flush()
		case quote == 0 && (r == '\'' || r == '"'):
			quote = r
			inToken = true
		case quote == r:
			quote = 0
			inToken = true
		default:
			current = append(current, r)
			inToken = true
		}
	}

	if escaped {
		current = append(current, '\\')
	}
	flush()

	return tokens
}

func ContainsHeredoc(tokens []string) bool {
	for _, token := range tokens {
		if token == "<<" || token == "<<-" {
			return true
		}
		for i := 0; i < len(token); i++ {
			if token[i] != '<' {
				continue
			}
			if i+1 < len(token) && token[i+1] == '<' {
				return true
			}
		}
	}
	return false
}
