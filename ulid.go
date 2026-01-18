package ulid

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"math/bits"
	"sync"
	"time"
	"unique"
	"weak"
)

const (
	// EncodedSize is the length of a text encoded ULID
	EncodedSize = 26

	// RawSize is the length of a binary encoded ULID
	RawSize = 16

	// MaxTime is the maximum Unix time in milliseconds that can be
	// represented in a ULID
	MaxTime = math.MaxUint64 >> 16
)

var (
	// ErrDataSize is returned when parsing or unmarshaling ULIDs with the wrong
	// data size
	ErrDataSize = errors.New("ulid: bad data size when unmarshaling")

	// ErrInvalidCharacters is returned when parsing or unmarshaling ULIDs with
	// invalid Base32 characters
	ErrInvalidCharacters = errors.New("ulid: bad data characters when unmarshaling")

	// ErrBufferSize is returned when marshaling ULIDs to a buffer of insufficient
	// size
	ErrBufferSize = errors.New("ulid: bad buffer size when marshaling")

	// ErrBigTime is returned when constructing a ULID with a time that is larger
	// than MaxTime
	ErrBigTime = errors.New("ulid: time too big")

	// ErrOverflow is returned when unmarshaling a ULID whose first character is
	// larger than 7, thereby exceeding the valid bit depth of 128
	ErrOverflow = errors.New("ulid: overflow when unmarshaling")

	// ErrMonotonicOverflow is returned by a Monotonic entropy source when
	// incrementing the previous ULID's entropy bytes would result in overflow
	ErrMonotonicOverflow = errors.New("ulid: monotonic entropy overflow")

	// ErrScanValue is returned when the value passed to scan cannot be unmarshaled
	// into the ULID
	ErrScanValue = errors.New("ulid: source value must be a string or byte slice")
)

// ULID is a 16 byte Universally Unique Lexicographically Sortable Identifier
type ULID [RawSize]byte

// Handle est un wrapper pour unique.Handle[ULID], permettant des comparaisons de pointeurs.
type Handle = unique.Handle[ULID]

// Handle retourne un handle unique pour cet ULID (interning).
func (id ULID) Handle() Handle {
	return unique.Make(id)
}

// Cache pour les strings via weak pointers pour éviter les re-allocations
// et permettre au GC de libérer la mémoire.
type stringRef struct{ s string }

var (
	textBufferPool = sync.Pool{New: func() any { b := make([]byte, EncodedSize); return &b }}

	// Cache global de strings pour les ULIDs via weak pointers.
	stringCache sync.Map // map[Handle]weak.Pointer[stringRef]
)

// New returns a ULID with the given Unix milliseconds timestamp and an optional
// entropy source. Use the Timestamp function to convert a time.Time to Unix
// milliseconds.
//
// ErrBigTime is returned when passing a timestamp bigger than MaxTime.
// Reading from the entropy source may also return an error.
//
// Safety for concurrent use is only dependent on the safety of the entropy source.
// New returns a ULID with the given Unix milliseconds timestamp and an optional
// entropy source. Use the Timestamp function to convert a time.Time to Unix
// milliseconds.
//
// ErrBigTime is returned when passing a timestamp bigger than MaxTime.
// Reading from the entropy source may also return an error.
//
// Safety for concurrent use is only dependent on the safety of the entropy source.
func New(ms uint64, entropy io.Reader) (ULID, error) {
	if ms > MaxTime {
		return ULID{}, ErrBigTime
	}

	var id ULID
	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)

	// Chemin fast-path : 0 allocation garanti car rand.Read est une fonction directe
	if entropy == nil || entropy == rand.Reader {
		if _, err := rand.Read(id[6:]); err != nil {
			return ULID{}, err
		}
		return id, nil
	}

	// Chemin slow-path isolé pour éviter d'influencer l'escape analysis du fast-path
	return newWithCustomEntropy(id, entropy)
}

//go:noinline
func newWithCustomEntropy(id ULID, entropy io.Reader) (ULID, error) {
	if _, err := io.ReadFull(entropy, id[6:]); err != nil {
		return ULID{}, err
	}
	return id, nil
}

// MustNew est inlinable pour de meilleures performances
func MustNew(ms uint64, entropy io.Reader) ULID {
	id, err := New(ms, entropy)
	if err != nil {
		panic(err)
	}
	return id
}

// Make est ultra-optimisé et inlinable
func Make() ULID {
	var id ULID
	ms := uint64(time.Now().UnixMilli())

	// Manuel inline pour éviter les overheads
	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)

	_, _ = rand.Read(id[6:])
	return id
}

// MakeWithTime returns a ULID with the given time and entropy from the
// default entropy source (crypto/rand.Reader).
func MakeWithTime(t time.Time) ULID {
	return MustNew(Timestamp(t), nil)
}

// Parse parses an encoded ULID, returning an error in case of failure.
//
// ErrDataSize is returned if the len(ulid) is different from EncodedSize.
// Invalid encodings produce undefined ULIDs. For a version that returns
// an error instead, see ParseStrict.
func Parse(s string) (id ULID, err error) {
	return parse([]byte(s), false)
}

// ParseStrict parses an encoded ULID, returning an error in case of failure.
//
// It is like Parse, but additionally validates that the parsed ULID consists
// only of valid base32 characters. It is slightly slower than Parse.
//
// ErrDataSize is returned if the len(ulid) is different from EncodedSize.
// ErrInvalidCharacters is returned if the parsed ULID contains invalid
// base32 characters.
func ParseStrict(s string) (id ULID, err error) {
	return parse([]byte(s), true)
}

func parse(v []byte, strict bool) (id ULID, err error) {
	if len(v) != EncodedSize {
		return id, ErrDataSize
	}

	// 6 bytes timestamp (48 bits)
	if dec[v[0]] == 0xFF ||
		dec[v[1]] == 0xFF ||
		dec[v[2]] == 0xFF ||
		dec[v[3]] == 0xFF ||
		dec[v[4]] == 0xFF ||
		dec[v[5]] == 0xFF ||
		dec[v[6]] == 0xFF ||
		dec[v[7]] == 0xFF ||
		dec[v[8]] == 0xFF ||
		dec[v[9]] == 0xFF {
		return id, ErrInvalidCharacters
	}

	// Validate base32 encoding
	if v[0] > '7' {
		return id, ErrOverflow
	}

	// Decode
	id[0] = (dec[v[0]] << 5) | dec[v[1]]
	id[1] = (dec[v[2]] << 3) | (dec[v[3]] >> 2)
	id[2] = (dec[v[3]] << 6) | (dec[v[4]] << 1) | (dec[v[5]] >> 4)
	id[3] = (dec[v[5]] << 4) | (dec[v[6]] >> 1)
	id[4] = (dec[v[6]] << 7) | (dec[v[7]] << 2) | (dec[v[8]] >> 3)
	id[5] = (dec[v[8]] << 5) | dec[v[9]]
	id[6] = (dec[v[10]] << 3) | (dec[v[11]] >> 2)
	id[7] = (dec[v[11]] << 6) | (dec[v[12]] << 1) | (dec[v[13]] >> 4)
	id[8] = (dec[v[13]] << 4) | (dec[v[14]] >> 1)
	id[9] = (dec[v[14]] << 7) | (dec[v[15]] << 2) | (dec[v[16]] >> 3)
	id[10] = (dec[v[16]] << 5) | dec[v[17]]
	id[11] = (dec[v[18]] << 3) | (dec[v[19]] >> 2)
	id[12] = (dec[v[19]] << 6) | (dec[v[20]] << 1) | (dec[v[21]] >> 4)
	id[13] = (dec[v[21]] << 4) | (dec[v[22]] >> 1)
	id[14] = (dec[v[22]] << 7) | (dec[v[23]] << 2) | (dec[v[24]] >> 3)
	id[15] = (dec[v[24]] << 5) | dec[v[25]]

	if strict {
		for _, c := range v {
			if dec[c] == 0xFF {
				return id, ErrInvalidCharacters
			}
		}
	}

	return id, nil
}

// String utilise un cache de weak pointers pour éviter les allocations répétées
// pour le même ULID, tout en restant sûr pour le GC.
func (id ULID) String() string {
	h := unique.Make(id)

	// Tentative de lecture du cache
	if v, ok := stringCache.Load(h); ok {
		if ref := v.(weak.Pointer[stringRef]).Value(); ref != nil {
			return ref.s
		}
	}

	// Génération de la string
	bufPtr := textBufferPool.Get().(*[]byte)
	buf := *bufPtr
	_ = id.MarshalTextTo(buf)
	s := string(buf)
	textBufferPool.Put(bufPtr)

	// Stockage dans le cache avec weak pointer
	ref := &stringRef{s: s}
	stringCache.Store(h, weak.Make(ref))

	return s
}

// MarshalBinary implements the encoding.BinaryMarshaler interface by
// returning the ULID as a byte slice.
func (id ULID) MarshalBinary() ([]byte, error) {
	ulid := make([]byte, RawSize)
	return ulid, id.MarshalBinaryTo(ulid)
}

// MarshalBinaryTo writes the binary encoding of the ULID to the given buffer.
// ErrBufferSize is returned when the len(dst) != RawSize.
func (id ULID) MarshalBinaryTo(dst []byte) error {
	if len(dst) != RawSize {
		return ErrBufferSize
	}

	copy(dst, id[:])
	return nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface by
// copying the passed data and converting it to a ULID. ErrDataSize is
// returned if the data length is different from RawSize.
func (id *ULID) UnmarshalBinary(data []byte) error {
	if len(data) != RawSize {
		return ErrDataSize
	}

	copy((*id)[:], data)
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface by
// returning the string encoded ULID.
func (id ULID) MarshalText() ([]byte, error) {
	ulid := make([]byte, EncodedSize)
	return ulid, id.MarshalTextTo(ulid)
}

// MarshalTextTo writes the ULID as a string to the given buffer.
// ErrBufferSize is returned when the len(dst) != EncodedSize.
func (id ULID) MarshalTextTo(dst []byte) error {
	if len(dst) != EncodedSize {
		return ErrBufferSize
	}

	// 10 byte timestamp
	dst[0] = enc[(id[0]&224)>>5]
	dst[1] = enc[id[0]&31]
	dst[2] = enc[(id[1]&248)>>3]
	dst[3] = enc[((id[1]&7)<<2)|((id[2]&192)>>6)]
	dst[4] = enc[(id[2]&62)>>1]
	dst[5] = enc[((id[2]&1)<<4)|((id[3]&240)>>4)]
	dst[6] = enc[((id[3]&15)<<1)|((id[4]&128)>>7)]
	dst[7] = enc[(id[4]&124)>>2]
	dst[8] = enc[((id[4]&3)<<3)|((id[5]&224)>>5)]
	dst[9] = enc[id[5]&31]

	// 16 bytes of entropy
	dst[10] = enc[(id[6]&248)>>3]
	dst[11] = enc[((id[6]&7)<<2)|((id[7]&192)>>6)]
	dst[12] = enc[(id[7]&62)>>1]
	dst[13] = enc[((id[7]&1)<<4)|((id[8]&240)>>4)]
	dst[14] = enc[((id[8]&15)<<1)|((id[9]&128)>>7)]
	dst[15] = enc[(id[9]&124)>>2]
	dst[16] = enc[((id[9]&3)<<3)|((id[10]&224)>>5)]
	dst[17] = enc[id[10]&31]
	dst[18] = enc[(id[11]&248)>>3]
	dst[19] = enc[((id[11]&7)<<2)|((id[12]&192)>>6)]
	dst[20] = enc[(id[12]&62)>>1]
	dst[21] = enc[((id[12]&1)<<4)|((id[13]&240)>>4)]
	dst[22] = enc[((id[13]&15)<<1)|((id[14]&128)>>7)]
	dst[23] = enc[(id[14]&124)>>2]
	dst[24] = enc[((id[14]&3)<<3)|((id[15]&224)>>5)]
	dst[25] = enc[id[15]&31]

	return nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface by
// parsing the data as string encoded ULID.
//
// ErrDataSize is returned if the len(v) is different from an encoded
// ULID's length. Invalid encodings produce undefined ULIDs.
func (id *ULID) UnmarshalText(v []byte) error {
	ulid, err := parse(v, false)
	if err != nil {
		return err
	}
	*id = ulid
	return nil
}

// Time returns the Unix time in milliseconds encoded in the ULID.
// Use the top level Time function to convert the returned value to
// a time.Time.
func (id ULID) Time() uint64 {
	return binary.BigEndian.Uint64(id[:8]) >> 16
}

// Timestamp converts a time.Time to Unix milliseconds.
func Timestamp(t time.Time) uint64 {
	return uint64(t.UnixMilli())
}

// Time converts Unix milliseconds in the format returned by the Timestamp
// function to a time.Time.
func Time(ms uint64) time.Time {
	s := int64(ms / 1e3)
	ns := int64((ms % 1e3) * 1e6)
	return time.Unix(s, ns)
}

// SetTime sets the time component of the ULID to the given Unix time
// in milliseconds.
func (id *ULID) SetTime(ms uint64) error {
	if ms > MaxTime {
		return ErrBigTime
	}

	binary.BigEndian.PutUint64(id[:8], ms<<16)
	return nil
}

// Entropy returns the entropy from the ULID.
func (id ULID) Entropy() []byte {
	e := make([]byte, 10)
	copy(e, id[6:])
	return e
}

// SetEntropy sets the ULID entropy to the passed byte slice.
// ErrDataSize is returned if len(e) != 10.
func (id *ULID) SetEntropy(e []byte) error {
	if len(e) != 10 {
		return ErrDataSize
	}

	copy(id[6:], e)
	return nil
}

// Compare returns an integer comparing id and other lexicographically.
// The result will be 0 if id==other, -1 if id < other, and +1 if id > other.
func (id ULID) Compare(other ULID) int {
	for i := 0; i < RawSize; i++ {
		if id[i] < other[i] {
			return -1
		}
		if id[i] > other[i] {
			return 1
		}
	}
	return 0
}

// Scan implements the sql.Scanner interface. It supports scanning
// a string or byte slice.
func (id *ULID) Scan(src interface{}) error {
	switch x := src.(type) {
	case nil:
		return nil
	case string:
		return id.UnmarshalText([]byte(x))
	case []byte:
		return id.UnmarshalText(x)
	}
	return ErrScanValue
}

// Value implements the sql/driver.Valuer interface, returning the ULID as a
// string.
func (id ULID) Value() (driver.Value, error) {
	return id.String(), nil
}

// IsZero returns true if the ULID is a zero value (all bytes are zero).
func (id ULID) IsZero() bool {
	return id == ULID{}
}

// Crockford's Base32 encoding
var enc = [32]byte{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'J', 'K',
	'M', 'N', 'P', 'Q', 'R', 'S', 'T', 'V', 'W', 'X',
	'Y', 'Z',
}

// dec is the inverse of enc, mapping ASCII bytes to their base32 values.
// 0xFF represents an invalid character.
var dec = [256]byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
	0x11, 0xFF, 0x12, 0x13, 0xFF, 0x14, 0x15, 0xFF,
	0x16, 0x17, 0x18, 0x19, 0x1A, 0xFF, 0x1B, 0x1C,
	0x1D, 0x1E, 0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
	0x11, 0xFF, 0x12, 0x13, 0xFF, 0x14, 0x15, 0xFF,
	0x16, 0x17, 0x18, 0x19, 0x1A, 0xFF, 0x1B, 0x1C,
	0x1D, 0x1E, 0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,

	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// Monotonic entropy source
type monoReader struct {
	io.Reader
	ms   uint64
	inc  uint64
	rand uint64
}

// MonotonicReader returns an io.Reader that generates monotonically increasing
// entropy bytes for a given timestamp. If the timestamp is the same, it will
// increment the previous entropy. If the timestamp is different, it will
// generate new random entropy.
//
// The returned reader is NOT safe for concurrent use.
func MonotonicReader(ms uint64, entropy io.Reader) io.Reader {
	if entropy == nil {
		entropy = rand.Reader
	}

	m := &monoReader{
		Reader: entropy,
		ms:     ms,
	}

	if err := binary.Read(entropy, binary.BigEndian, &m.rand); err != nil {
		panic(err)
	}

	m.inc = m.rand

	return m
}

func (m *monoReader) Read(p []byte) (n int, err error) {
	if len(p) != 10 {
		return 0, ErrDataSize
	}

	if m.ms == 0 {
		if err := binary.Read(m.Reader, binary.BigEndian, &m.rand); err != nil {
			return 0, err
		}
		m.inc = m.rand
	} else {
		m.inc++
		if m.inc < m.rand {
			return 0, ErrMonotonicOverflow
		}
	}

	binary.BigEndian.PutUint64(p[:8], m.inc)
	copy(p[:], p[2:10])

	return 10, nil
}

// MonotonicEntropy is a convenience function that returns a MonotonicReader
// for the given time.
func MonotonicEntropy(t time.Time, entropy io.Reader) io.Reader {
	return MonotonicReader(Timestamp(t), entropy)
}

// Nil is the nil ULID (all zeros)
var Nil ULID

// Zero returns a zero-valued ULID
func Zero() ULID {
	return Nil
}

// IsNil returns true if the ULID is the nil ULID (all zeros)
func (id ULID) IsNil() bool {
	return id == Nil
}

// Bytes returns the ULID as a byte slice
func (id ULID) Bytes() []byte {
	return id[:]
}

// Less returns true if id is lexicographically less than other
func (id ULID) Less(other ULID) bool {
	return id.Compare(other) < 0
}

// Greater returns true if id is lexicographically greater than other
func (id ULID) Greater(other ULID) bool {
	return id.Compare(other) > 0
}

// Equal returns true if id is equal to other
func (id ULID) Equal(other ULID) bool {
	return id == other
}

// LeadingZeros returns the number of leading zero bits in the ULID
func (id ULID) LeadingZeros() int {
	for i, b := range id {
		if b != 0 {
			return i*8 + bits.LeadingZeros8(b)
		}
	}
	return 128
}

// TrailingZeros returns the number of trailing zero bits in the ULID
func (id ULID) TrailingZeros() int {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] != 0 {
			return (len(id)-i-1)*8 + bits.TrailingZeros8(id[i])
		}
	}
	return 128
}

// MarshalJSON est maintenant optimisé à 1 seule allocation (le buffer de retour).
func (id ULID) MarshalJSON() ([]byte, error) {
	// 26 caractères + 2 guillemets = 28 octets
	res := make([]byte, 28)
	res[0] = '"'
	_ = id.MarshalTextTo(res[1:27])
	res[27] = '"'
	return res, nil
}

// UnmarshalJSON est maintenant Garanti 0 allocation.
func (id *ULID) UnmarshalJSON(data []byte) error {
	// Vérification de taille exacte pour éviter les overheads
	if len(data) != 28 || data[0] != '"' || data[27] != '"' {
		return ErrDataSize
	}
	// On parse directement la tranche interne
	return id.UnmarshalText(data[1:27])
}
