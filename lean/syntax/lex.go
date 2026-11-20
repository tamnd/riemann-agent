// Package syntax reads a Lean 4 file as text, without Lean. It finds the imports, the namespaces, the open commands and the declarations, and the identifiers in each declaration. The result is not exact: the reader does not elaborate, so it cannot resolve dot notation or the names that a macro makes. Spec 04.2 gives the rules.
package syntax

import (
	"unicode"
	"unicode/utf8"
)

// tokKind is the kind of a token.
type tokKind uint8

const (
	tIdent  tokKind = iota // an identifier, with its dots: Foo.bar
	tSymbol                // one or two characters of punctuation: ( ) := : | @[
	tDoc                   // a doc comment /-- ... -/
	tNumber                // a number literal
	tString                // a string or character literal
)

// token is one token of a file. The lexer drops white space and the comments that are not doc comments.
type token struct {
	kind tokKind
	text string
	off  int  // the byte offset of the first character
	end  int  // the byte offset after the last character
	line int  // 1-based
	col  int  // 0-based, in bytes
	bol  bool // the token is the first token on its line
	dot  bool // an identifier that has a dot before it with no space, as in .inl or (f x).foo
}

// isIdentStart reports whether r can start an identifier. Lean takes the letters, _, and the letter-like symbols. λ, Π and Σ are not letters in Lean.
func isIdentStart(r rune) bool {
	if r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') {
		return true
	}
	if r < 0x80 || r == 'λ' || r == 'Π' || r == 'Σ' {
		return false
	}
	if 0x2100 <= r && r <= 0x214F { // letter-like symbols: ℂ ℕ ℝ ℤ ℚ
		return true
	}
	if 0x1D49C <= r && r <= 0x1D59F { // script and double-struck letters: 𝒜 𝔸
		return true
	}
	return unicode.IsLetter(r)
}

// isIdentRest reports whether r can go on an identifier.
func isIdentRest(r rune) bool {
	if isIdentStart(r) || ('0' <= r && r <= '9') || r == '\'' || r == '!' || r == '?' {
		return true
	}
	// Subscripts: ₀ to ₜ, ᵢ to ᵪ, ⱼ.
	return (0x2080 <= r && r <= 0x209C) || (0x1D62 <= r && r <= 0x1D6A) || r == 0x2C7C
}

// lex splits src into tokens.
func lex(src []byte) []token {
	var toks []token
	line, lineStart := 1, 0
	bol := true
	i := 0
	n := len(src)
	emit := func(k tokKind, start, end int) {
		toks = append(toks, token{kind: k, text: string(src[start:end]), off: start, end: end, line: line, col: start - lineStart, bol: bol})
		bol = false
	}
	// newlines counts the line breaks in src[a:b] and moves the line state.
	newlines := func(a, b int) {
		for j := a; j < b; j++ {
			if src[j] == '\n' {
				line++
				lineStart = j + 1
				bol = true
			}
		}
	}
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
			lineStart = i
			bol = true
			continue
		case c == ' ' || c == '\t' || c == '\r':
			i++
			continue
		case c == '-' && i+1 < n && src[i+1] == '-':
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		case c == '/' && i+1 < n && src[i+1] == '-':
			start := i
			doc := i+2 < n && src[i+2] == '-' && !(i+3 < n && src[i+3] == '/')
			depth := 0
			for i < n {
				if src[i] == '/' && i+1 < n && src[i+1] == '-' {
					depth++
					i += 2
					continue
				}
				if src[i] == '-' && i+1 < n && src[i+1] == '/' {
					depth--
					i += 2
					if depth == 0 {
						break
					}
					continue
				}
				i++
			}
			if doc {
				sl, sc, sb := line, start-lineStart, bol
				toks = append(toks, token{kind: tDoc, text: string(src[start:i]), off: start, end: i, line: sl, col: sc, bol: sb})
				bol = false
			}
			newlines(start, i)
			continue
		case c == '"':
			start := i
			i++
			for i < n && src[i] != '"' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i = min(i+1, n)
			emit(tString, start, i)
			newlines(start, i)
			continue
		case c == '\'' && i+2 < n && (src[i+2] == '\'' || src[i+1] == '\\') && !prevIdent(src, i):
			// A character literal 'a' or '\n'. A prime after an identifier is part of the identifier.
			start := i
			i++
			if src[i] == '\\' {
				i += 2
			} else {
				_, w := utf8.DecodeRune(src[i:])
				i += w
			}
			for i < n && src[i] != '\'' && src[i] != '\n' {
				i++
			}
			i = min(i+1, n)
			emit(tString, start, i)
			continue
		case '0' <= c && c <= '9':
			start := i
			for i < n && (isDigitish(src[i])) {
				i++
			}
			emit(tNumber, start, i)
			continue
		}
		r, w := utf8.DecodeRune(src[i:])
		if isIdentStart(r) || r == '«' {
			start := i
			i = scanIdent(src, i)
			dot := start > 0 && src[start-1] == '.' && (start < 2 || src[start-2] != '.')
			emit(tIdent, start, i)
			toks[len(toks)-1].dot = dot
			continue
		}
		start := i
		if i+1 < n {
			two := string(src[i : i+2])
			switch two {
			case ":=", "@[", "::", "=>", "<|", "|>", "..":
				i += 2
				emit(tSymbol, start, i)
				continue
			}
		}
		i += w
		emit(tSymbol, start, i)
	}
	return toks
}

func isDigitish(c byte) bool {
	return ('0' <= c && c <= '9') || c == '_' || c == 'x' || c == 'b' || c == 'o' || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// prevIdent reports whether the byte before i ends an identifier.
func prevIdent(src []byte, i int) bool {
	if i == 0 {
		return false
	}
	r, _ := utf8.DecodeLastRune(src[:i])
	return isIdentRest(r) || r == '»'
}

// scanIdent reads an identifier with dots from i: a.b.c, «a b».c, x₁. A dot that is not before an identifier part ends it, so h.1 gives h.
func scanIdent(src []byte, i int) int {
	n := len(src)
	for {
		if i < n && src[i] == 0xC2 && i+1 < n && src[i+1] == 0xAB { // «
			j := i + 2
			for j < n && !(src[j] == 0xC2 && j+1 < n && src[j+1] == 0xBB) && src[j] != '\n' {
				j++
			}
			if j >= n || src[j] == '\n' {
				return j
			}
			i = j + 2
		} else {
			r, w := utf8.DecodeRune(src[i:])
			if !isIdentStart(r) {
				return i
			}
			i += w
			for i < n {
				r, w := utf8.DecodeRune(src[i:])
				if !isIdentRest(r) {
					break
				}
				i += w
			}
		}
		if i+1 < n && src[i] == '.' {
			r, _ := utf8.DecodeRune(src[i+1:])
			if isIdentStart(r) || r == '«' {
				i++
				continue
			}
		}
		return i
	}
}
