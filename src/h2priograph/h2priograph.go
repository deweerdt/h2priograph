package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lucasb-eyer/go-colorful"
)

const (
	OUT = iota
	IN_RECV_DATA
	IN_HEADERS
	IN_PUSH_PROMISE
)

type Stream struct {
	priority   int
	parent_sid int
	sid        int
	url        string
	extension  string
	base       string
	exclusive  bool
	is_push    bool
	done       bool /* set when we find the END_STREAM */
	children   []*Stream
}

func ColorToString(c colorful.Color) string {
	return fmt.Sprintf("%f %f %f", c.R, c.G, c.B)
}

func isbrowny(l, a, b float64) bool {
	h, c, L := colorful.LabToHcl(l, a, b)
	return h > 250.0 && c > 0.5 && L > 0.5
}
func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func fillPathInfo(s *Stream, url string) {
	s.url = url
	s.base = filepath.Base(s.url)
	s.extension = filepath.Ext(s.url)
	if len(s.extension) > 1 {
		s.extension = s.extension[1:]
		qm := strings.Index(s.extension, "?")
		if qm > 0 {
			s.extension = s.extension[:qm]
		}
	}
}
func main() {
	var file = flag.String("file", "", "filename")

	flag.Parse()

	state := OUT
	var i int
	var s *Stream
	streams := make([]*Stream, 0)
	line_nr := 0

	f, err := os.Open(*file)
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	/* index streams by id */
	streams_by_id := make(map[int]*Stream)
	/* fictious 0 stream */
	stream0 := &Stream{}
	streams = append(streams, stream0)
	streams_by_id[0] = stream0
	stream0.done = true

	scanner := bufio.NewScanner(f)
	saw_fin := false
	for scanner.Scan() {
		line := scanner.Text()
		line_nr++

		extractInt := func(match, line string) (int, bool) {
			i := strings.Index(line, match)
			if i > 0 {
				nr, err := strconv.Atoi(line[i+len(match):])
				if err != nil {
					println(fmt.Sprintf("cannot parse %s: %s, line: %d", match, line, line_nr))
					os.Exit(0)
				}
				return nr, true
			}
			return 0, false
		}

	OOH:
		switch state {
		case OUT:
			if strings.HasPrefix(line, "t=") {
				if strings.Contains(line, "HTTP2_SESSION_SEND_HEADERS") {
					state = IN_HEADERS
					s = &Stream{}
				}
				if strings.Contains(line, "HTTP2_SESSION_RECV_DATA") {
					saw_fin = false
					state = IN_RECV_DATA
				}
				if strings.Contains(line, "HTTP2_SESSION_RECV_PUSH_PROMISE") {
					state = IN_PUSH_PROMISE
					s = &Stream{}
					s.is_push = true
				}
			}
		case IN_RECV_DATA:
			if strings.HasPrefix(line, "t=") {
				state = OUT
				goto OOH
			}
			i = strings.Index(line, "--> fin = true")
			if i > 0 {
				saw_fin = true
			}
			i, found := extractInt("--> stream_id = ", line)
			if found {
				streams_by_id[i].done = saw_fin
			}
		case IN_PUSH_PROMISE:
			if strings.HasPrefix(line, "t=") {
				state = OUT
				if s.exclusive && !streams_by_id[s.parent_sid].done {
					/* transfer parent's streams to current stream */
					s.children = streams_by_id[s.parent_sid].children
					streams_by_id[s.parent_sid].children = []*Stream{s}
				} else {
					streams_by_id[s.parent_sid].children = append(streams_by_id[s.parent_sid].children, s)
				}
				streams = append(streams, s)
				goto OOH
			}
			i = strings.Index(line, ":path: ")
			if i > 0 {
				fillPathInfo(s, line[i+7:])
				continue
			}
			i, found := extractInt("--> promised_stream_id = ", line)
			if found {
				s.sid = i
				streams_by_id[s.sid] = s
				continue
			}
		case IN_HEADERS:
			if strings.HasPrefix(line, "t=") {
				state = OUT
				if s.exclusive && !streams_by_id[s.parent_sid].done {
					/* transfer parent's streams to current stream */
					s.children = streams_by_id[s.parent_sid].children
					streams_by_id[s.parent_sid].children = []*Stream{s}
				} else {
					streams_by_id[s.parent_sid].children = append(streams_by_id[s.parent_sid].children, s)
				}
				streams = append(streams, s)
				goto OOH
			}
			i = strings.Index(line, ":path: ")
			if i > 0 {
				fillPathInfo(s, line[i+7:])

			}
			if strings.HasPrefix(line, "t=") {
				state = OUT
				streams = append(streams, s)
				goto OOH
			}
			i, found := extractInt("parent_stream_id = ", line)
			if found {
				s.parent_sid = i
				continue
			}
			i = strings.Index(line, "exclusive = true")
			if i > 0 {
				s.exclusive = true
			}
			i, found = extractInt("weight = ", line)
			if found {
				s.priority = i
				continue
			}
			/* add --> to distinguish from has_priority */
			i, found = extractInt("--> priority = ", line)
			if found {
				s.priority = i
				continue
			}
			i, found = extractInt("--> stream_id = ", line)
			if found {
				s.sid = i
				streams_by_id[s.sid] = s
				continue
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	rand.Seed(2)

	fmt.Printf("digraph G {\n")
	colors := make(map[string]colorful.Color)
	extensions_count_h := make(map[string]struct{})
	extensions_count := 10
	for _, s := range streams {
		_, ok := extensions_count_h[s.extension]
		if !ok {
			extensions_count++
			extensions_count_h[s.extension] = struct{}{}
		}
	}
	palette, err := colorful.SoftPaletteEx(extensions_count, colorful.SoftPaletteSettings{isbrowny, 50, true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	for _, s := range streams {
		c, ok := colors[s.extension]
		if !ok {
			c = palette[0]
			palette = palette[1:]
			colors[s.extension] = c
		}
		label := s.extension
		if label == "" {
			label = s.url[:min(len(s.base), 40)]
		}
		label = fmt.Sprintf("%s - sid:%d - %s - %d", s.base[:min(len(s.base), 40)], s.sid, label, s.priority)
		shape := "ellipse"
		if s.is_push {
			shape = "larrow"
		}
		fmt.Printf("%d [style=filled,label=\"%s\", color=\"%s\", shape=\"%s\"];\n", s.sid, label, ColorToString(c), shape)
		//fmt.Printf("%d -> %d;\n", s.parent_sid, s.sid)
		for _, c := range s.children {
			fmt.Printf("%d -> %d;\n", s.sid, c.sid)
		}
	}
	fmt.Printf("}\n")
	return
}
