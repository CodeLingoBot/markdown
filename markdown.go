/*  Original C version https://github.com/jgm/peg-markdown/
 *	Copyright 2008 John MacFarlane (jgm at berkeley dot edu).
 *
 *  Modifications and translation from C into Go
 *  based on markdown_lib.c and parsing_functions.c
 *	Copyright 2010 Michael Teichgräber (mt at wmipf dot de)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License or the MIT
 *  license.  See LICENSE for details.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 */

package markdown

import (
	"bytes"
	"io"
	"log"
	"strings"
)

// Markdown Options:
type Options struct {
	Smart        bool
	Notes        bool
	FilterHTML   bool
	FilterStyles bool
	Dlists       bool
}

type Parser struct {
	yy           yyParser
	preformatBuf *bytes.Buffer
}

// NewParser creates an instance of a parser. It can be reused
// so that stacks and buffers need not be allocated anew for
// each Markdown call.
func NewParser(opt *Options) (p *Parser) {
	p = new(Parser)
	if opt != nil {
		p.yy.state.extension = *opt
	}
	p.yy.Init()
	p.preformatBuf = bytes.NewBuffer(make([]byte, 0, 32768))
	return
}

type Formatter interface {
	FormatBlock(*element)
	Finish()
}

// Markdown parses input from an io.Reader into a tree, and sends
// parsed blocks to a Formatter
func (p *Parser) Markdown(src io.Reader, f Formatter) {
	s := p.preformat(src)

	p.parseRule(ruleReferences, s)
	if p.yy.extension.Notes {
		p.parseRule(ruleNotes, s)
	}

L:
	for {
		tree := p.parseRule(ruleDocblock, s)
		s = p.yy.ResetBuffer("")
		tree = p.processRawBlocks(tree)
		f.FormatBlock(tree)
		switch s {
		case "", "\n", "\r\n", "\n\n", "\r\n\n", "\n\n\n", "\r\n\n\n":
			break L
		}
	}
	f.Finish()
}

func (p *Parser) parseRule(rule int, s string) (tree *element) {
	if p.yy.ResetBuffer(s) != "" {
		log.Fatalf("Buffer not empty")
	}
	if err := p.yy.Parse(rule); err != nil {
		log.Fatalln("markdown:", err)
	}
	switch rule {
	case ruleDoc, ruleDocblock:
		tree = p.yy.state.tree
		p.yy.state.tree = nil
	}
	return
}

/* process_raw_blocks - traverses an element list, replacing any RAW elements with
 * the result of parsing them as markdown text, and recursing into the children
 * of parent elements.  The result should be a tree of elements without any RAWs.
 */
func (p *Parser) processRawBlocks(input *element) *element {

	for current := input; current != nil; current = current.next {
		if current.key == RAW {
			/* \001 is used to indicate boundaries between nested lists when there
			 * is no blank line.  We split the string by \001 and parse
			 * each chunk separately.
			 */
			current.key = LIST
			current.children = nil
			listEnd := &current.children
			for _, contents := range strings.Split(current.contents.str, "\001") {
				if list := p.parseRule(ruleDoc, contents); list != nil {
					*listEnd = list
					for list.next != nil {
						list = list.next
					}
					listEnd = &list.next
				}
			}
			current.contents.str = ""
		}
		if current.children != nil {
			current.children = p.processRawBlocks(current.children)
		}
	}
	return input
}

const (
	TABSTOP = 4
)

/* preformat - allocate and copy text buffer while
 * performing tab expansion.
 */
func (p *Parser) preformat(r io.Reader) (s string) {
	charstotab := TABSTOP
	buf := make([]byte, 32768)

	b := p.preformatBuf
	b.Reset()
	for {
		n, err := r.Read(buf)
		if err != nil {
			break
		}
		i0 := 0
		for i := range buf[:n] {
			switch buf[i] {
			case '\t':
				b.Write(buf[i0:i])
				for ; charstotab > 0; charstotab-- {
					b.WriteByte(' ')
				}
				i0 = i + 1
			case '\n':
				b.Write(buf[i0 : i+1])
				i0 = i + 1
				charstotab = TABSTOP
			default:
				charstotab--
			}
			if charstotab == 0 {
				charstotab = TABSTOP
			}
		}
		b.Write(buf[i0:n])
	}

	b.WriteString("\n\n")
	return b.String()
}
