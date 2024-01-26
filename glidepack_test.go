package glidepack

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/DataDog/zstd"
	"github.com/pierrec/lz4/v4"
)

var v = map[Algorithm]Validator{
	DEFLATE: &DeflateValidator{},
	GZIP:    &GzipValidator{},
	LZ4:     &LZ4Validator{},
	ZSTD:    &ZstdValidator{},
}

type Validator interface {
	Validate(input string, output []byte, t *testing.T)
}

type GzipValidator struct{}

func (v *GzipValidator) Validate(input string, output []byte, t *testing.T) {
	reader, err := gzip.NewReader(bytes.NewReader(output))
	if err != nil {
		t.Fatalf("gzip reader initialization failed: %v", err)
		return
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("error reading gzip data: %v", err)
		return
	}

	if string(decompressed) != input {
		t.Errorf("gzip mismatch\n***expected***\n%q:%d bytes\n\n***received***\n%q:%d",
			input, len(input), string(decompressed), len(decompressed))
	}
}

type DeflateValidator struct{}

func (v *DeflateValidator) Validate(input string, output []byte, t *testing.T) {
	reader := flate.NewReader(bytes.NewReader(output))
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("error reading deflate data: %v", err)
		return
	}

	if string(decompressed) != input {
		t.Errorf("deflate mismatch\n***expected***\n%q:%d bytes\n\n***received***\n%q:%d",
			input, len(input), string(decompressed), len(decompressed))
	}
}

type ZstdValidator struct{}

func (v *ZstdValidator) Validate(input string, output []byte, t *testing.T) {
	decompressed, err := zstd.Decompress(nil, output)
	if err != nil {
		t.Fatalf("error decompressing zstd data: %v", err)
		return
	}

	if string(decompressed) != input {
		t.Errorf("zstd mismatch\n***expected***\n%q:%d bytes\n\n***received***\n%q:%d",
			input, len(input), string(decompressed), len(decompressed))
	}
}

type LZ4Validator struct{}

func (v *LZ4Validator) Validate(input string, output []byte, t *testing.T) {
	r := lz4.NewReader(bytes.NewReader(output))
	decompressed := make([]byte, len(input)*2)
	_, err := r.Read(decompressed)
	if err != nil {
		t.Fatalf("error decompressing lz4 data: %v", err)
		return
	}

	if string(decompressed) != input {
		t.Errorf("lz4 mismatch\n***expected***\n%q:%d bytes\n\n***received***\n%q:%d",
			input, len(input), string(decompressed), len(decompressed))
	}
}

func runStringCompressTest(str string, alg Algorithm, validator Validator, t *testing.T) {
	b := new(bytes.Buffer)

	z := NewWriter(b)
	z.Apply(AlgorithmOption(alg)) // Set the compression algorithm
	// if algo == LZ4 {
	// 	TODO: To fix lz4, we need to be able to pass config options to the handler

	// }
	_, err := z.Write([]byte(str))
	if err != nil {
		t.Errorf("TestFail: '%v'", err)
	}
	z.Close()

	validator.Validate(str, b.Bytes(), t)
}

type Input struct {
	name string
	buf  string
}

func TestCompressString(t *testing.T) {
	inputs := []Input{
		{"Short String", "Hello World\n"},
		{"Long String", strings.Repeat("Hello World\n", 1501/len("Hello World\n"))},
	}

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			for _, algo := range []Algorithm{GZIP, ZSTD} {
				t.Run(algo.String(), func(t *testing.T) {
					validator, ok := v[algo]
					if !ok {
						t.Fatalf("No validator found for algorithm: %v", algo)
					}

					runStringCompressTest(input.buf, algo, validator, t)
				})
			}
		})
	}
}

type testCase struct {
	algorithm     Algorithm
	strategies    []StrategyType
	expectedError error
	validate      func(input string, output []byte, t *testing.T)
}

func TestHandlers(t *testing.T) {
	tests := []testCase{
		{
			algorithm:     GZIP,
			strategies:    []StrategyType{ISAL, QAT, DEFAULT},
			expectedError: nil,
			validate:      v[GZIP].Validate,
		},
		{
			algorithm:     ZSTD,
			strategies:    []StrategyType{QAT, DEFAULT},
			expectedError: nil,
			validate:      v[ZSTD].Validate,
		},
		{
			algorithm:     ZSTD,
			strategies:    []StrategyType{ISAL},
			expectedError: errNoWorkingStrategies,
			validate:      nil,
		},

		// Add more test cases here for each combination of algorithm and strategy
	}

	for _, tc := range tests {
		input := "Hello world"
		for _, strategy := range tc.strategies {
			t.Run(tc.algorithm.String()+"With"+strategy.String(), func(t *testing.T) {
				b := new(bytes.Buffer)
				z := NewWriter(b)
				z.Apply(AlgorithmOption(tc.algorithm))
				z.SetPolicy(func(pp *PolicyParameters) []StrategyType {
					return []StrategyType{strategy}
				})

				_, err := z.Write([]byte(input))
				if err != tc.expectedError {
					t.Errorf("Test failed for algorithm '%s' with strategy '%s': %v", tc.algorithm, strategy, err)
				} else if err == nil {
					tc.validate(input, b.Bytes(), t)
				}
			})
		}
	}
}

func TestMultipleWritesISAL(t *testing.T) {
	b := new(bytes.Buffer)
	input := "Hello world"
	midpoint := len(input) / 2
	alg := GZIP
	z := NewWriter(b)
	z.Apply(AlgorithmOption(alg))
	z.SetPolicy(func(pp *PolicyParameters) []StrategyType {
		return []StrategyType{ISAL}
	})
	_, err := z.Write([]byte(input[:midpoint]))
	if err != nil {
		t.Errorf("TestFail: ISAL write failed with '%s'", err.Error())
	}
	_, err = z.Write([]byte(input[midpoint:]))
	if err != nil {
		t.Errorf("TestFail: ISAL second write failed with '%s'", err.Error())
	}
	v[alg].Validate(input, b.Bytes(), t)
}

func TestMultipleStrategyWrites(t *testing.T) {
	b := new(bytes.Buffer)
	input := strings.Repeat("Hello World\n", 2000/len("Hello World\n"))
	midpoint := 100
	alg := GZIP
	z := NewWriter(b)
	z.Apply(AlgorithmOption(alg))
	// this write should hit isal
	_, err := z.Write([]byte(input[:midpoint]))
	if err != nil {
		t.Errorf("TestFail: ISAL write failed with '%s'", err.Error())
	}
	// This write should hit qat
	_, err = z.Write([]byte(input[midpoint:]))
	if err != nil {
		t.Errorf("TestFail: ISAL second write failed with '%s'", err.Error())
	}
	v[alg].Validate(input, b.Bytes(), t)
}

func TestWriterApply(t *testing.T) {
	b := bytes.NewBuffer([]byte("Hello World"))
	z := NewWriter(b)

	err := z.Apply(CompressionLevelOption(5))
	if err != nil {
		t.Errorf("TestFail: initialization failure, received '%v'", err)

	}
	z.Close()
}

func TestCloseThenWrite(t *testing.T) {
	s := "Test String..."
	b := bytes.NewBuffer([]byte(s))
	d := new(bytes.Buffer)
	z := NewWriter(d)
	z.Close()
	n, err := io.Copy(z, b)
	if n > 0 || err == nil {
		t.Error("TestFail:", n, "bytes copied on a closed Writer err:", err)
	}
}

func TestWriterReset(t *testing.T) {
	// Sample data to write
	dataToWrite := []byte("sample data for writing")
	resetCount := 10
	// Buffer to hold the compressed data
	bufLength := 128 * 1024
	b := bytes.NewBuffer(make([]byte, 0, bufLength))

	// Create a new Writer
	z := NewWriter(b)

	for i := 0; i < resetCount; i++ {
		// Write data to the Writer
		nw, err := z.Write(dataToWrite)
		if err != nil {
			t.Fatalf("TestInit: error writing data: '%v'", err)
		}

		if nw == 0 {
			t.Fatalf("TestInit: no data written.")
		}

		// Close the Writer
		err = z.Close()
		if err != nil || z.closed == false {
			t.Fatalf("TestInit: error failed to close Writer err:'%v' closed:%v", err, z.closed)
		}

		// Reset the buffer and the Writer for the next iteration
		b.Reset()
		z.Reset(b)
		if err != nil || z.closed == true {
			t.Fatalf("TestInit: error failed to reset QAT err:'%v' closed:'%v'", err, z.closed)
		}
	}
}

func runStringDecompressTest(str string, algo Algorithm, t *testing.T) {
	b := new(bytes.Buffer)
	if algo == GZIP {
		f := gzip.NewWriter(b)
		f.Write([]byte(str))
		err := f.Close()
		if err != nil {
			t.Fatalf("TestInit: error failed to close flate writer '%v'", err)
		}
	}
	if algo == ZSTD {
		f := zstd.NewWriter(b)
		f.Write([]byte(str))
		err := f.Close()
		if err != nil {
			t.Fatalf("TestInit: error failed to close flate writer '%v'", err)
		}
	}

	z := NewReader(b)
	out := new(bytes.Buffer)

	z.Apply(AlgorithmOption(algo))

	_, err := io.Copy(out, z)

	if err != nil && err != io.EOF {
		t.Fatalf("Decompression failed: '%v'", err)
	}

	stringCompare(str, out, t)

}

func stringCompare(str string, b *bytes.Buffer, t *testing.T) {
	if b.String() != str {
		t.Errorf("mismatch\n***expected***\n%q:%d bytes\n\n ***received***\n%q:%d", str, len(str), b.String(), len(b.String()))
	}
}

func TestDecompressString(t *testing.T) {
	inputs := []Input{
		{"Short String", "Hello World"},
		{"Long String", strings.Repeat("Hello World\n", 1501/len("Hello World\n"))},
		// Add more test cases as needed
	}

	for _, input := range inputs {
		t.Run(input.name, func(t *testing.T) {
			for _, algo := range []Algorithm{GZIP, ZSTD} {
				t.Run(algo.String(), func(t *testing.T) {
					runStringDecompressTest(input.buf, algo, t)
				})
			}
		})
	}
}

func TestMultipleQATDecompressString(t *testing.T) {
	alg := GZIP
	wnum := 9
	str := strings.Repeat("Hello World\n", 65536/len("Hello World\n"))

	for i := 0; i < wnum; i++ {
		b := new(bytes.Buffer)
		f := gzip.NewWriter(b)
		f.Write([]byte(str))
		err := f.Close()
		if err != nil {
			t.Fatalf("TestInit: error failed to close flate writer '%v'", err)
		}

		z := NewReader(b)
		out := new(bytes.Buffer)

		z.Apply(AlgorithmOption(alg))
		z.SetPolicy(func(pp *PolicyParameters) []StrategyType {
			return []StrategyType{QAT}
		})

		_, err = io.Copy(out, z)

		if err != nil && err != io.EOF {
			t.Fatalf("Decompression failed: '%v'", err)
		}

		stringCompare(str, out, t)
	}

}
