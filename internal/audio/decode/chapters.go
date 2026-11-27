package decode

import (
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// Chapter is a named point in a file, as an audiobook's chapters are.
type Chapter struct {
	Title string
	Start time.Duration
}

// maxChapters is the most chapters a file may list; real books have
// hundreds at most.
const maxChapters = 10000

// Chapters reads the chapters an MP4 file (.m4b, .m4a) lists: its QuickTime
// chapter track, as Apple's tools write, else its Nero chapter list (a
// moov/udta/chpl box). They come by start time. A file with neither, or of
// another format, has none.
func Chapters(path string) ([]Chapter, error) {
	switch strings.ToLower(path[strings.LastIndexByte(path, '.')+1:]) {
	case "m4a", "m4b", "mp4":
	default:
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return readChapters(f, info.Size())
}

// readChapters reads an MP4 file's chapters, as Chapters says.
func readChapters(r io.ReaderAt, size int64) ([]Chapter, error) {
	var nero []Chapter
	tracks := map[uint32]box{} // every trak, by its ID
	var wanted []uint32        // the chapter tracks an audio track names
	err := eachBox(r, 0, size, func(b box) error {
		if b.typ != "moov" {
			return nil
		}
		return eachBox(r, b.start, b.end, func(b box) error {
			switch b.typ {
			case "trak":
				id, chap, err := trackRefs(r, b)
				if err != nil {
					return err
				}
				tracks[id] = b
				wanted = append(wanted, chap...)
			case "udta":
				c, err := find(r, b, "chpl")
				if err != nil {
					return err
				}
				p, err := c.payload(r, "chpl")
				if err != nil {
					return err
				}
				nero = neroChapters(p)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	for _, id := range wanted {
		if t, ok := tracks[id]; ok {
			if cs, err := textChapters(r, t, size); err == nil && len(cs) > 0 {
				return cs, nil
			}
		}
	}
	return nero, nil
}

// trackRefs reads a trak's ID and the chapter tracks its tref/chap names.
func trackRefs(r io.ReaderAt, trak box) (id uint32, chap []uint32, err error) {
	top, err := find(r, trak, "tkhd", "tref")
	if err != nil {
		return 0, nil, err
	}
	tkhd, err := top.payload(r, "tkhd")
	if err != nil {
		return 0, nil, err
	}
	b := be{p: tkhd}
	if b.uint(1) == 1 {
		b.bytes(3 + 16)
	} else {
		b.bytes(3 + 8)
	}
	id = uint32(b.uint(4))
	tref, err := top.find(r, "tref", "chap")
	if err != nil {
		return 0, nil, err
	}
	p, err := tref.payload(r, "chap")
	if err != nil {
		return 0, nil, err
	}
	for i := 0; i+4 <= len(p) && len(chap) < 16; i += 4 {
		chap = append(chap, uint32(be{p: p[i : i+4]}.peek()))
	}
	return id, chap, nil
}

// neroChapters reads a chpl payload: a version, flags, a count, then each
// chapter's start in 100 ns units and its title.
func neroChapters(p []byte) []Chapter {
	b := be{p: p}
	version := b.uint(1)
	b.bytes(3) // flags
	if version != 0 {
		b.bytes(4) // reserved
	}
	n := int(b.uint(1))
	var out []Chapter
	for ; n > 0 && !b.bad; n-- {
		start := b.uint(8)
		title := b.bytes(int(b.uint(1)))
		if b.bad || start > uint64(maxTicks)*10 {
			break
		}
		out = append(out, Chapter{Title: clean(title), Start: time.Duration(start) * 100})
	}
	return ordered(out)
}

// textChapters reads a chapter track: a text track whose samples are the
// chapters' titles, each lasting as long as its chapter.
func textChapters(r io.ReaderAt, trak box, size int64) ([]Chapter, error) {
	top, err := find(r, trak, "mdia")
	if err != nil {
		return nil, err
	}
	mdia, err := top.find(r, "mdia", "mdhd", "minf")
	if err != nil {
		return nil, err
	}
	mdhd, err := mdia.payload(r, "mdhd")
	if err != nil {
		return nil, err
	}
	unit, _ := timescale(mdhd)
	if unit <= 0 || unit > maxScale {
		return nil, errBadMP4
	}
	minf, err := mdia.find(r, "minf", "stbl")
	if err != nil {
		return nil, err
	}
	stbl, err := minf.find(r, "stbl", "stts", "stsz", "stsc", "stco", "co64")
	if err != nil {
		return nil, err
	}
	var p [5][]byte
	for i, typ := range []string{"stts", "stsz", "stsc", "stco", "co64"} {
		if p[i], err = stbl.payload(r, typ); err != nil {
			return nil, err
		}
	}
	sizes := sampleSizes(p[1], size)
	chunks := chunkOffsets(p[3], 4)
	if p[4] != nil {
		chunks = chunkOffsets(p[4], 8)
	}
	offsets := frameOffsets(chunks, p[2], sizes)
	starts := sampleStarts(p[0], len(sizes))
	if len(sizes) == 0 || len(sizes) > maxChapters || len(offsets) != len(sizes) || len(starts) != len(sizes) {
		return nil, errBadMP4
	}
	out := make([]Chapter, len(sizes))
	for i := range sizes {
		out[i] = Chapter{Start: time.Duration(starts[i] * int64(time.Second) / unit)}
		n := min(int(sizes[i]), 2+1024)
		if n < 2 {
			continue
		}
		s := make([]byte, n)
		if err := readAt(r, s, offsets[i]); err != nil {
			return nil, err
		}
		l := int(be{p: s[:2]}.peek())
		out[i].Title = clean(s[2:min(2+l, n)])
	}
	return ordered(out), nil
}

// sampleStarts reads an stts payload: when each of n samples starts, in the
// track's timescale.
func sampleStarts(p []byte, n int) []int64 {
	b := be{p: p}
	b.bytes(4)
	out := make([]int64, 0, n)
	var t int64
	for runs := b.uint(4); runs > 0 && !b.bad && len(out) < n; runs-- {
		count, delta := int(b.uint(4)), int64(b.uint(4))
		for ; count > 0 && len(out) < n; count-- {
			out = append(out, t)
			if t += delta; t > maxTicks {
				return nil
			}
		}
	}
	if b.bad {
		return nil
	}
	return out
}

// clean is a chapter title as text: UTF-16 with its byte-order mark, or
// UTF-8, trimmed.
func clean(p []byte) string {
	if len(p) >= 2 && (p[0] == 0xFE && p[1] == 0xFF || p[0] == 0xFF && p[1] == 0xFE) {
		big := p[0] == 0xFE
		u := make([]uint16, 0, len(p)/2)
		for i := 2; i+1 < len(p); i += 2 {
			if big {
				u = append(u, uint16(p[i])<<8|uint16(p[i+1]))
			} else {
				u = append(u, uint16(p[i+1])<<8|uint16(p[i]))
			}
		}
		return strings.TrimSpace(string(utf16.Decode(u)))
	}
	return strings.TrimSpace(strings.ToValidUTF8(strings.TrimRight(string(p), "\x00"), string(utf8.RuneError)))
}

// ordered drops chapters out of order, as a damaged list may have.
func ordered(cs []Chapter) []Chapter {
	out := cs[:0]
	for _, c := range cs {
		if len(out) == 0 || c.Start > out[len(out)-1].Start {
			out = append(out, c)
		}
	}
	if len(out) > maxChapters {
		out = out[:maxChapters]
	}
	return out
}
