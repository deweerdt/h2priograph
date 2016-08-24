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
	OUT_OF_HEADERS = iota
	IN_HEADERS
	HAS_PRIO
)

type Stream struct {
	priority   int
	parent_sid int
	sid        int
	url        string
	extension  string
	base       string
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
func main() {
	var file = flag.String("file", "", "filename")

	flag.Parse()

	state := OUT_OF_HEADERS
	var i int
	var s *Stream
	streams := make([]*Stream, 0)
	line_nr := 0

	f, err := os.Open(*file)
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		line_nr++

	OOH:
		switch state {
		case OUT_OF_HEADERS:
			if strings.HasPrefix(line, "t=") {
				if strings.Contains(line, "HTTP2_SESSION_SEND_HEADERS") {
					state = IN_HEADERS
					s = &Stream{}
				}
			}
		case IN_HEADERS:
			if strings.HasPrefix(line, "t=") {
				state = OUT_OF_HEADERS
				streams = append(streams, s)
				goto OOH
			}
			if strings.Contains(line, "has_priority = true") {
				state = HAS_PRIO
			}
		case HAS_PRIO:
			i = strings.Index(line, ":path: ")
			if i > 0 {
				s.url = line[i+7:]
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
			if strings.HasPrefix(line, "t=") {
				state = OUT_OF_HEADERS
				streams = append(streams, s)
				goto OOH
			}
			i = strings.Index(line, "parent_stream_id = ")
			if i > 0 {
				nr, err := strconv.Atoi(line[i+19:])
				if err != nil {
					println(fmt.Sprintf("cannot parse parent_stream_id: %s %s, line: %d", line[i+19:], line, line_nr))
					os.Exit(0)
				}
				s.parent_sid = nr
				continue
			}
			i = strings.Index(line, "weight = ")
			if i > 0 {
				nr, err := strconv.Atoi(line[i+9:])
				if err != nil {
					println(fmt.Sprintf("cannot parse weight: %s, line: %d", line, line_nr))
					os.Exit(0)
				}
				s.priority = nr
				continue
			}
			i = strings.Index(line, "priority = ")
			if i > 0 {
				nr, err := strconv.Atoi(line[i+11:])
				if err != nil {
					println(fmt.Sprintf("cannot parse priority: %s, line: %d", line, line_nr))
					os.Exit(0)
				}
				s.priority = nr
				continue
			}
			i = strings.Index(line, "stream_id = ")
			if i > 0 {
				nr, err := strconv.Atoi(line[i+12:])
				if err != nil {
					println(fmt.Sprintf("cannot parse stream_id: %s, line: %d", line, line_nr))
					os.Exit(0)
				}
				s.sid = nr
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
		fmt.Printf("%d [style=filled,label=\"%s\", color=\"%s\"];\n", s.sid, label, ColorToString(c))
		fmt.Printf("%d -> %d;\n", s.parent_sid, s.sid)
	}
	fmt.Printf("}\n")
	return
}
