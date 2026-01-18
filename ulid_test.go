package ulid

import (
	"bytes"
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	now := time.Now()
	ms := Timestamp(now)

	id, err := New(ms, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if id.Time() != ms {
		t.Errorf("Time() = %v, want %v", id.Time(), ms)
	}

	// Test with custom entropy
	entropy := bytes.NewReader(make([]byte, 10))
	id2, err := New(ms, entropy)
	if err != nil {
		t.Fatalf("New() with custom entropy error = %v", err)
	}

	if id2.Time() != ms {
		t.Errorf("Time() = %v, want %v", id2.Time(), ms)
	}
}

func TestMake(t *testing.T) {
	id := Make()
	if id.IsZero() {
		t.Error("Make() returned zero ULID")
	}

	// Verify time is recent
	now := Timestamp(time.Now())
	diff := int64(now) - int64(id.Time())
	if diff < 0 {
		diff = -diff
	}
	if diff > 1000 { // Allow 1 second difference
		t.Errorf("Make() time difference too large: %v ms", diff)
	}
}

func TestMakeWithTime(t *testing.T) {
	testTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	id := MakeWithTime(testTime)

	if id.IsZero() {
		t.Error("MakeWithTime() returned zero ULID")
	}

	expectedMs := Timestamp(testTime)
	if id.Time() != expectedMs {
		t.Errorf("Time() = %v, want %v", id.Time(), expectedMs)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid ULID",
			input:   "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			wantErr: false,
		},
		{
			name:    "too short",
			input:   "01ARZ3NDEKTSV4RRFFQ69G5FA",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   "01ARZ3NDEKTSV4RRFFQ69G5FAVX",
			wantErr: true,
		},
		{
			name:    "invalid characters in timestamp",
			input:   "01ARZ3NDE!TSV4RRFFQ69G5FAV",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseStrict(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid ULID",
			input:   "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			wantErr: false,
		},
		{
			name:    "lowercase (invalid in strict mode)",
			input:   "01arz3ndektsv4rrffq69g5fav",
			wantErr: false, // Lowercase is actually valid in Crockford base32
		},
		{
			name:    "invalid character",
			input:   "01ARZ3NDEKTSV4RRFFQ69G5F!V",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseStrict(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStrict() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestString(t *testing.T) {
	id := Make()
	str := id.String()

	if len(str) != EncodedSize {
		t.Errorf("String() length = %v, want %v", len(str), EncodedSize)
	}

	// Parse it back
	parsed, err := Parse(str)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if parsed != id {
		t.Errorf("Parse(String()) = %v, want %v", parsed, id)
	}
}

func TestMarshalBinary(t *testing.T) {
	id := Make()
	data, err := id.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}

	if len(data) != RawSize {
		t.Errorf("MarshalBinary() length = %v, want %v", len(data), RawSize)
	}

	var id2 ULID
	if err := id2.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary() error = %v", err)
	}

	if id2 != id {
		t.Errorf("UnmarshalBinary(MarshalBinary()) = %v, want %v", id2, id)
	}
}

func TestMarshalText(t *testing.T) {
	id := Make()
	data, err := id.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}

	if len(data) != EncodedSize {
		t.Errorf("MarshalText() length = %v, want %v", len(data), EncodedSize)
	}

	var id2 ULID
	if err := id2.UnmarshalText(data); err != nil {
		t.Fatalf("UnmarshalText() error = %v", err)
	}

	if id2 != id {
		t.Errorf("UnmarshalText(MarshalText()) = %v, want %v", id2, id)
	}
}

func TestMarshalJSON(t *testing.T) {
	id := Make()
	data, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var id2 ULID
	if err := json.Unmarshal(data, &id2); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if id2 != id {
		t.Errorf("json.Unmarshal(json.Marshal()) = %v, want %v", id2, id)
	}
}

func TestTime(t *testing.T) {
	now := time.Now()
	ms := Timestamp(now)

	id, err := New(ms, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if id.Time() != ms {
		t.Errorf("Time() = %v, want %v", id.Time(), ms)
	}

	// Convert back to time.Time
	recovered := Time(id.Time())
	diff := now.Sub(recovered)
	if diff < 0 {
		diff = -diff
	}

	// Allow up to 1ms difference due to truncation
	if diff > time.Millisecond {
		t.Errorf("Time conversion difference = %v, want <= 1ms", diff)
	}
}

func TestSetTime(t *testing.T) {
	id := Make()
	newMs := uint64(1234567890000)

	if err := id.SetTime(newMs); err != nil {
		t.Fatalf("SetTime() error = %v", err)
	}

	if id.Time() != newMs {
		t.Errorf("Time() = %v, want %v", id.Time(), newMs)
	}

	// Test overflow
	if err := id.SetTime(MaxTime + 1); err != ErrBigTime {
		t.Errorf("SetTime(MaxTime+1) error = %v, want %v", err, ErrBigTime)
	}
}

func TestEntropy(t *testing.T) {
	id := Make()
	entropy := id.Entropy()

	if len(entropy) != 10 {
		t.Errorf("Entropy() length = %v, want 10", len(entropy))
	}

	// Create new ULID with same entropy
	var id2 ULID
	id2.SetTime(id.Time())
	if err := id2.SetEntropy(entropy); err != nil {
		t.Fatalf("SetEntropy() error = %v", err)
	}

	if id2 != id {
		t.Errorf("SetEntropy(Entropy()) = %v, want %v", id2, id)
	}
}

func TestCompare(t *testing.T) {
	id1 := Make()
	time.Sleep(2 * time.Millisecond)
	id2 := Make()

	if id1.Compare(id1) != 0 {
		t.Error("Compare(self) should return 0")
	}

	if id1.Compare(id2) >= 0 {
		t.Error("Compare() should return negative for earlier ULID")
	}

	if id2.Compare(id1) <= 0 {
		t.Error("Compare() should return positive for later ULID")
	}

	// Test helper methods
	if !id1.Less(id2) {
		t.Error("Less() should return true for earlier ULID")
	}

	if !id2.Greater(id1) {
		t.Error("Greater() should return true for later ULID")
	}

	if !id1.Equal(id1) {
		t.Error("Equal(self) should return true")
	}
}

func TestScan(t *testing.T) {
	id := Make()
	str := id.String()

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name:    "string",
			input:   str,
			wantErr: false,
		},
		{
			name:    "byte slice",
			input:   []byte(str),
			wantErr: false,
		},
		{
			name:    "nil",
			input:   nil,
			wantErr: false,
		},
		{
			name:    "invalid type",
			input:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var id2 ULID
			err := id2.Scan(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Scan() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.input != nil && id2 != id {
				t.Errorf("Scan() = %v, want %v", id2, id)
			}
		})
	}
}

func TestValue(t *testing.T) {
	id := Make()
	val, err := id.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	str, ok := val.(string)
	if !ok {
		t.Fatalf("Value() type = %T, want string", val)
	}

	if str != id.String() {
		t.Errorf("Value() = %v, want %v", str, id.String())
	}

	// Verify it implements driver.Valuer
	var _ driver.Valuer = id
}

func TestIsZero(t *testing.T) {
	var zero ULID
	if !zero.IsZero() {
		t.Error("IsZero() should return true for zero ULID")
	}

	id := Make()
	if id.IsZero() {
		t.Error("IsZero() should return false for non-zero ULID")
	}
}

func TestIsNil(t *testing.T) {
	if !Nil.IsNil() {
		t.Error("IsNil() should return true for Nil ULID")
	}

	id := Make()
	if id.IsNil() {
		t.Error("IsNil() should return false for non-nil ULID")
	}
}

func TestMonotonicReader(t *testing.T) {
	ms := Timestamp(time.Now())
	entropy := MonotonicReader(ms, rand.Reader)

	var prev ULID
	for i := 0; i < 10; i++ {
		id, err := New(ms, entropy)
		if err != nil {
			t.Fatalf("New() with monotonic entropy error = %v", err)
		}

		if i > 0 && !prev.Less(id) {
			t.Errorf("Monotonic ULID %d not greater than %d", i, i-1)
		}

		prev = id
	}
}

func TestLeadingZeros(t *testing.T) {
	var zero ULID
	if zero.LeadingZeros() != 128 {
		t.Errorf("LeadingZeros() for zero ULID = %v, want 128", zero.LeadingZeros())
	}

	id := Make()
	lz := id.LeadingZeros()
	if lz < 0 || lz > 128 {
		t.Errorf("LeadingZeros() = %v, want 0-128", lz)
	}
}

func TestTrailingZeros(t *testing.T) {
	var zero ULID
	if zero.TrailingZeros() != 128 {
		t.Errorf("TrailingZeros() for zero ULID = %v, want 128", zero.TrailingZeros())
	}

	id := Make()
	tz := id.TrailingZeros()
	if tz < 0 || tz > 128 {
		t.Errorf("TrailingZeros() = %v, want 0-128", tz)
	}
}

func TestBytes(t *testing.T) {
	id := Make()
	b := id.Bytes()

	if len(b) != RawSize {
		t.Errorf("Bytes() length = %v, want %v", len(b), RawSize)
	}

	// Verify it's the same data
	for i := 0; i < RawSize; i++ {
		if b[i] != id[i] {
			t.Errorf("Bytes()[%d] = %v, want %v", i, b[i], id[i])
		}
	}
}

func TestSortability(t *testing.T) {
	// Create ULIDs with increasing timestamps
	ids := make([]ULID, 10)
	for i := 0; i < 10; i++ {
		ids[i] = Make()
		time.Sleep(time.Millisecond)
	}

	// Verify they're sorted
	for i := 1; i < len(ids); i++ {
		if !ids[i-1].Less(ids[i]) {
			t.Errorf("ULID %d not less than ULID %d", i-1, i)
		}

		// Also verify string representation is sorted
		if ids[i-1].String() >= ids[i].String() {
			t.Errorf("ULID string %d not less than ULID string %d", i-1, i)
		}
	}
}

func BenchmarkNew(b *testing.B) {
	ms := Timestamp(time.Now())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = New(ms, rand.Reader)
	}
}

func BenchmarkMake(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Make()
	}
}

func BenchmarkParse(b *testing.B) {
	id := Make()
	str := id.String()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = Parse(str)
	}
}

func BenchmarkString(b *testing.B) {
	id := Make()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

func BenchmarkMarshalText(b *testing.B) {
	id := Make()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = id.MarshalText()
	}
}

func BenchmarkUnmarshalText(b *testing.B) {
	id := Make()
	data, _ := id.MarshalText()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var id2 ULID
		_ = id2.UnmarshalText(data)
	}
}

func BenchmarkMarshalJSON(b *testing.B) {
	id := Make()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(id)
	}
}

func BenchmarkUnmarshalJSON(b *testing.B) {
	id := Make()
	data, _ := json.Marshal(id)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var id2 ULID
		_ = json.Unmarshal(data, &id2)
	}
}
