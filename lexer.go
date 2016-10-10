package dhcp

import (
	"bufio"
	"bytes"
	"net"
	"strconv"
	"unicode"
)

type lexer struct {
	line     int
	r        *bufio.Reader
	buffer   []*lexToken
	prev     *lexToken
	readPrev bool
}

func newLexer(r *bufio.Reader) *lexer {
	return &lexer{r: r, line: 1}
}

// This function will make the lexer reread the previous token. This can
// only be used to reread one token.
func (l *lexer) unread() {
	l.readPrev = true
}

func (l *lexer) all() []*lexToken {
	tokens := make([]*lexToken, 0)
	for {
		tok := l.next()
		if tok.token == EOF {
			break
		}
		tokens = append(tokens, tok)
	}
	return tokens
}

func (l *lexer) next() *lexToken {
	if l.readPrev {
		l.readPrev = false
		return l.prev
	}

	if len(l.buffer) > 0 {
		tok := l.buffer[0]
		l.buffer = l.buffer[1:]
		l.prev = tok
		return tok
	}

	var tok []*lexToken // Some consumes produce multiple tokens

	for {
		c, err := l.r.ReadByte()
		if err != nil {
			return &lexToken{token: EOF}
		}

		if c == '"' {
			tok = l.consumeString() // Start after double quote
			break
		} else if isNumber(c) {
			l.r.UnreadByte()
			tok = l.consumeNumeric()
			break
		} else if c == '\n' {
			l.line++
		} else if c == '#' {
			line := l.consumeLine()
			tok = []*lexToken{
				&lexToken{
					token: COMMENT,
					value: string(line),
				},
			}
			break
		} else if isLetter(c) {
			l.r.UnreadByte()
			tok = l.consumeIdent()
			break
		}
	}

	// Ensure all produced tokens have a line number
	for _, t := range tok {
		t.line = l.line
	}

	// This function only returns one token, if more were created,
	// add them to a buffer to be returned later
	if len(tok) > 1 {
		l.buffer = tok[1:]
	}

	l.prev = tok[0]
	return tok[0]
}

func (l *lexer) consumeString() []*lexToken {
	buf := bytes.Buffer{}
	for {
		b, err := l.r.ReadByte()
		if err != nil {
			return nil
		}
		if b == '"' {
			break
		}
		buf.WriteByte(b)
	}
	return []*lexToken{&lexToken{token: STRING, value: buf.String()}}
}

func (l *lexer) consumeLine() []byte {
	buf := bytes.Buffer{}
	for {
		b, err := l.r.ReadByte()
		if err != nil {
			return nil
		}
		if b == '\n' {
			l.r.UnreadByte()
			break
		}
		buf.WriteByte(b)
	}
	return buf.Bytes()
}

func (l *lexer) consumeNumeric() []*lexToken {
	buf := bytes.Buffer{}
	dotCount := 0
	hasSlash := false

	for {
		b, err := l.r.ReadByte()
		if err != nil {
			return nil
		}
		if isNumber(b) {
			buf.WriteByte(b)
			continue
		} else if b == '.' {
			buf.WriteByte(b)
			dotCount++
			continue
		} else if b == '/' {
			buf.WriteByte(b)
			hasSlash = true
			continue
		}
		l.r.UnreadByte()
		break
	}

	toks := make([]*lexToken, 1)
	toks[0] = &lexToken{}
	if hasSlash && dotCount == 3 { // CIDR notation
		ip, network, err := net.ParseCIDR(buf.String())
		if err != nil {
			toks[0].token = ILLEGAL
		} else {
			toks[0].token = IP_ADDRESS
			toks[0].value = ip
			t := &lexToken{
				token: IP_ADDRESS,
				value: net.IP(network.Mask),
			}
			toks = append(toks, t)
		}
	} else if dotCount == 3 { // IP Address
		ip := net.ParseIP(buf.String())
		if ip == nil {
			toks[0].token = ILLEGAL
		} else {
			toks[0].token = IP_ADDRESS
			toks[0].value = ip
		}
	} else if dotCount == 0 { // Number
		num, err := strconv.Atoi(buf.String())
		if err != nil {
			toks[0].token = ILLEGAL
		} else {
			toks[0].token = NUMBER
			toks[0].value = num
		}
	}
	return toks
}

func (l *lexer) consumeIdent() []*lexToken {
	buf := bytes.Buffer{}
	for {
		b, err := l.r.ReadByte()
		if err != nil {
			return nil
		}
		if isWhitespace(b) {
			l.r.UnreadByte()
			break
		}
		buf.WriteByte(b)
	}
	tok := &lexToken{token: lookup(buf.String()), value: buf.String()}
	return []*lexToken{tok}
}

func isNumber(b byte) bool     { return unicode.IsDigit(rune(b)) }
func isLetter(b byte) bool     { return unicode.IsLetter(rune(b)) }
func isWhitespace(b byte) bool { return unicode.IsSpace(rune(b)) }
