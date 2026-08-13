package migrations

import (
	"fmt"
	"strings"
)

// splitMigrationSQLStatements validates the lightweight statement boundaries used
// by the project-owned migration catalog and returns deterministic, trimmed SQL
// statements. It is intentionally a conservative scanner, not a SQL dialect
// parser: semicolons terminate statements only outside quoted strings and
// comments, single-quoted string literals escape doubled single quotes,
// double-quoted identifiers escape doubled double quotes, and unterminated
// quotes/comments fail closed.
func splitMigrationSQLStatements(body string) ([]string, error) {
	const (
		stateNormal = iota
		stateSingleQuote
		stateDoubleQuote
		stateLineComment
		stateBlockComment
	)

	state := stateNormal
	statementStart := 0
	pendingTerminator := -1
	statements := make([]string, 0)

	emit := func(end int) {
		statement := strings.TrimSpace(body[statementStart:end])
		if sqlFragmentHasExecutableText(statement) {
			statements = append(statements, statement)
		}
		statementStart = end
		pendingTerminator = -1
	}

	for i := 0; i < len(body); {
		c := body[i]

		switch state {
		case stateNormal:
			if pendingTerminator >= 0 {
				switch {
				case c == ' ' || c == '\t' || c == '\r':
					i++
					continue
				case c == '\n':
					emit(i + 1)
					i++
					continue
				case c == '-' && i+1 < len(body) && body[i+1] == '-':
					state = stateLineComment
					i += 2
					continue
				default:
					emit(pendingTerminator)
					continue
				}
			}

			switch {
			case c == '\'':
				state = stateSingleQuote
				i++
			case c == '"':
				state = stateDoubleQuote
				i++
			case c == '-' && i+1 < len(body) && body[i+1] == '-':
				state = stateLineComment
				i += 2
			case c == '/' && i+1 < len(body) && body[i+1] == '*':
				state = stateBlockComment
				i += 2
			case c == ';':
				pendingTerminator = i + 1
				i++
			default:
				i++
			}

		case stateSingleQuote:
			if c == '\'' {
				if i+1 < len(body) && body[i+1] == '\'' {
					i += 2
					continue
				}
				state = stateNormal
			}
			i++

		case stateDoubleQuote:
			if c == '"' {
				if i+1 < len(body) && body[i+1] == '"' {
					i += 2
					continue
				}
				state = stateNormal
			}
			i++

		case stateLineComment:
			if c == '\n' {
				state = stateNormal
				if pendingTerminator >= 0 {
					emit(i + 1)
				}
			}
			i++

		case stateBlockComment:
			if c == '*' && i+1 < len(body) && body[i+1] == '/' {
				state = stateNormal
				i += 2
				continue
			}
			i++
		}
	}

	switch state {
	case stateSingleQuote:
		return nil, fmt.Errorf("%w: unterminated single-quoted string", ErrInvalidCatalog)
	case stateDoubleQuote:
		return nil, fmt.Errorf("%w: unterminated double-quoted identifier", ErrInvalidCatalog)
	case stateBlockComment:
		return nil, fmt.Errorf("%w: unterminated block comment", ErrInvalidCatalog)
	}

	if pendingTerminator >= 0 {
		emit(len(body))
	} else if sqlFragmentHasExecutableText(body[statementStart:]) {
		return nil, fmt.Errorf("%w: SQL statement is missing a terminating semicolon", ErrInvalidCatalog)
	}
	if len(statements) == 0 {
		return nil, fmt.Errorf("%w: SQL body has no terminated statements", ErrInvalidCatalog)
	}
	return statements, nil
}

func sqlFragmentHasExecutableText(fragment string) bool {
	const (
		stateNormal = iota
		stateSingleQuote
		stateDoubleQuote
		stateLineComment
		stateBlockComment
	)
	state := stateNormal
	for i := 0; i < len(fragment); {
		c := fragment[i]
		switch state {
		case stateNormal:
			switch {
			case c == '\'':
				return true
			case c == '"':
				return true
			case c == '-' && i+1 < len(fragment) && fragment[i+1] == '-':
				state = stateLineComment
				i += 2
				continue
			case c == '/' && i+1 < len(fragment) && fragment[i+1] == '*':
				state = stateBlockComment
				i += 2
				continue
			case c == ';' || c == ' ' || c == '\t' || c == '\r' || c == '\n':
				i++
				continue
			default:
				return true
			}
		case stateLineComment:
			if c == '\n' {
				state = stateNormal
			}
		case stateBlockComment:
			if c == '*' && i+1 < len(fragment) && fragment[i+1] == '/' {
				state = stateNormal
				i += 2
				continue
			}
		case stateSingleQuote, stateDoubleQuote:
			return true
		}
		i++
	}
	return false
}
