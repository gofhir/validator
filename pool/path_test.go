package pool

import (
	"sync"
	"testing"
)

func TestPathBuilder_Basic(t *testing.T) {
	pb := AcquirePathBuilder()
	defer pb.Release()

	pb.WriteString("Patient")
	pb.WriteByte('.')
	pb.WriteString("name")

	if got := pb.String(); got != "Patient.name" {
		t.Errorf("String() = %q; want %q", got, "Patient.name")
	}
}

func TestPathBuilder_Append(t *testing.T) {
	pb := AcquirePathBuilder()
	defer pb.Release()

	pb.Append("Patient", "name", "given")

	if got := pb.String(); got != "Patient.name.given" {
		t.Errorf("String() = %q; want %q", got, "Patient.name.given")
	}
}

func TestPathBuilder_AppendWithDot(t *testing.T) {
	pb := AcquirePathBuilder()
	defer pb.Release()

	pb.WriteString("Patient")
	pb.AppendWithDot("name")
	pb.AppendWithDot("given")

	if got := pb.String(); got != "Patient.name.given" {
		t.Errorf("String() = %q; want %q", got, "Patient.name.given")
	}

	// Test when buffer is empty
	pb.Reset()
	pb.AppendWithDot("Patient")
	if got := pb.String(); got != "Patient" {
		t.Errorf("String() with empty buffer = %q; want %q", got, "Patient")
	}
}

func TestPathBuilder_AppendIndex(t *testing.T) {
	pb := AcquirePathBuilder()
	defer pb.Release()

	pb.WriteString("Patient.name")
	pb.AppendIndex(0)

	if got := pb.String(); got != "Patient.name[0]" {
		t.Errorf("String() = %q; want %q", got, "Patient.name[0]")
	}

	pb.AppendWithDot("given")
	pb.AppendIndex(1)

	if got := pb.String(); got != "Patient.name[0].given[1]" {
		t.Errorf("String() = %q; want %q", got, "Patient.name[0].given[1]")
	}
}

func TestPathBuilder_Reset(t *testing.T) {
	pb := AcquirePathBuilder()
	defer pb.Release()

	pb.WriteString("Patient.name")
	pb.Reset()

	if pb.Len() != 0 {
		t.Errorf("Len() after Reset = %d; want 0", pb.Len())
	}

	pb.WriteString("Observation")
	if got := pb.String(); got != "Observation" {
		t.Errorf("String() after Reset = %q; want %q", got, "Observation")
	}
}

func TestPathBuilder_Bytes(t *testing.T) {
	pb := AcquirePathBuilder()
	defer pb.Release()

	pb.WriteString("Patient")
	bytes := pb.Bytes()

	if string(bytes) != "Patient" {
		t.Errorf("Bytes() = %q; want %q", string(bytes), "Patient")
	}
}

func TestPathBuilder_NilRelease(t *testing.T) {
	var pb *PathBuilder
	pb.Release() // Should not panic
}

func TestBuildPath(t *testing.T) {
	path := BuildPath(func(b *PathBuilder) {
		b.Append("Patient", "name")
		b.AppendIndex(0)
		b.AppendWithDot("given")
	})

	if path != "Patient.name[0].given" {
		t.Errorf("BuildPath = %q; want %q", path, "Patient.name[0].given")
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		segments []string
		want     string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"Patient"}, "Patient"},
		{[]string{"Patient", "name"}, "Patient.name"},
		{[]string{"Patient", "name", "given"}, "Patient.name.given"},
	}

	for _, tt := range tests {
		got := JoinPath(tt.segments...)
		if got != tt.want {
			t.Errorf("JoinPath(%v) = %q; want %q", tt.segments, got, tt.want)
		}
	}
}

func TestAppendArrayIndex(t *testing.T) {
	got := AppendArrayIndex("Patient.name", 2)
	want := "Patient.name[2]"
	if got != want {
		t.Errorf("AppendArrayIndex = %q; want %q", got, want)
	}
}

func TestPathBuilder_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pb := AcquirePathBuilder()
			pb.Append("Patient", "name")
			pb.AppendIndex(i)
			_ = pb.String()
			pb.Release()
		}(i)
	}

	wg.Wait()
}

func BenchmarkPathBuilder_Simple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pb := AcquirePathBuilder()
		pb.Append("Patient", "name", "given")
		_ = pb.String()
		pb.Release()
	}
}

func BenchmarkPathBuilder_Complex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pb := AcquirePathBuilder()
		pb.Append("Bundle", "entry")
		pb.AppendIndex(0)
		pb.AppendWithDot("resource")
		pb.AppendWithDot("name")
		pb.AppendIndex(0)
		pb.AppendWithDot("given")
		pb.AppendIndex(0)
		_ = pb.String()
		pb.Release()
	}
}

func BenchmarkBuildPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = BuildPath(func(pb *PathBuilder) {
			pb.Append("Patient", "name")
			pb.AppendIndex(0)
			pb.AppendWithDot("given")
		})
	}
}

func BenchmarkJoinPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = JoinPath("Patient", "name", "given")
	}
}

// Compare with naive string concatenation
func BenchmarkStringConcat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = "Patient" + "." + "name" + "." + "given"
	}
}
