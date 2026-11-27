package decode

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"
	"time"
)

func u32(vs ...uint32) []byte {
	var out []byte
	for _, v := range vs {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	return out
}

func full(typ string, body ...[]byte) []byte { // a full box: version and flags first
	return mkbox(typ, append([][]byte{u32(0)}, body...)...)
}

// chapterFile is an MP4 file with an audio track naming text track 2, whose
// samples are titles, and a Nero list that says otherwise.
func chapterFile(titles []string, starts []uint32, end uint32) []byte {
	ftyp := mkbox("ftyp", []byte("M4B "))
	var mdat []byte
	var sizes []byte
	for _, t := range titles {
		s := binary.BigEndian.AppendUint16(nil, uint16(len(t)))
		s = append(s, t...)
		sizes = append(sizes, u32(uint32(len(s)))...)
		mdat = append(mdat, s...)
	}
	mdatBox := mkbox("mdat", mdat)
	dataAt := uint32(len(ftyp) + 8)
	var stts []byte
	for i := range starts {
		next := end
		if i+1 < len(starts) {
			next = starts[i+1]
		}
		stts = append(stts, u32(1, next-starts[i])...)
	}
	text := mkbox("trak",
		full("tkhd", u32(0, 0, 2)),
		mkbox("mdia",
			full("mdhd", u32(0, 0, 1000, end)),
			mkbox("minf", mkbox("stbl",
				full("stts", u32(uint32(len(starts))), stts),
				full("stsz", u32(0, uint32(len(titles))), sizes),
				full("stsc", u32(1, 1, uint32(len(titles)), 1)),
				full("stco", u32(1, dataAt))))))
	audio := mkbox("trak", full("tkhd", u32(0, 0, 1)), mkbox("tref", mkbox("chap", u32(2))))
	nero := mkbox("udta", mkbox("chpl", []byte{1, 0, 0, 0, 0, 0, 0, 0, 1}, u64(0), []byte{4}, []byte("Nero")))
	return slices.Concat(ftyp, mdatBox, mkbox("moov", audio, text, nero))
}

func u64(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }

func TestChapterTrack(t *testing.T) {
	f := chapterFile([]string{"Opening Credits", "Chapter 1", "Chapter 2"}, []uint32{0, 15000, 1830500}, 4000000)
	cs, err := readChapters(bytes.NewReader(f), int64(len(f)))
	if err != nil {
		t.Fatal(err)
	}
	want := []Chapter{{"Opening Credits", 0}, {"Chapter 1", 15 * time.Second}, {"Chapter 2", 1830500 * time.Millisecond}}
	if !slices.Equal(cs, want) {
		t.Errorf("chapters = %+v, want %+v", cs, want)
	}
}

func TestNeroChapters(t *testing.T) {
	p := slices.Concat([]byte{1, 0, 0, 0, 0, 0, 0, 0, 2},
		u64(0), []byte{5}, []byte("Intro"),
		u64(600*1e7), []byte{2}, []byte("Ch"))
	cs := neroChapters(p)
	want := []Chapter{{"Intro", 0}, {"Ch", 10 * time.Minute}}
	if !slices.Equal(cs, want) {
		t.Errorf("chapters = %+v", cs)
	}
	f := slices.Concat(mkbox("ftyp", []byte("M4A ")), mkbox("moov", mkbox("udta", mkbox("chpl", p))))
	if cs, _ := readChapters(bytes.NewReader(f), int64(len(f))); !slices.Equal(cs, want) {
		t.Errorf("from the file: %+v", cs)
	}
}

func TestChaptersJunk(t *testing.T) {
	neroChapters([]byte{1, 0, 0, 0, 0, 0, 0, 0, 200, 1, 2}) // a count past the end
	if got := clean([]byte{0xFE, 0xFF, 0, 'H', 0, 'i'}); got != "Hi" {
		t.Errorf("UTF-16 = %q", got)
	}
	if cs := ordered([]Chapter{{"a", 0}, {"b", time.Second}, {"c", time.Second}, {"d", 0}}); len(cs) != 2 {
		t.Errorf("ordered = %+v", cs)
	}
}

func FuzzReadChapters(f *testing.F) {
	f.Add(chapterFile([]string{"A", "B"}, []uint32{0, 5000}, 9000))
	f.Fuzz(func(t *testing.T, b []byte) { readChapters(bytes.NewReader(b), int64(len(b))) })
}
