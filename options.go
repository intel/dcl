package glidepack

import "errors"

type Option func(a applier) error

var (
	ErrParamCompressionLevel = errors.New("compression parameter invalid")
	ErrApplyInvalidType      = errors.New("cannot apply parameters to this object")
	ErrParamAlgorithm        = errors.New("algorithm parameter invalid")
)

type applier interface {
	Apply(...Option) error
}

func CompressionLevelOption(level int) Option {
	return func(a applier) error {
		if level <= 0 {
			return ErrParamCompressionLevel
		}

		switch z := a.(type) {
		case *Writer:
			z.p.level = level
		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}

func AlgorithmOption(alg Algorithm) Option {
	return func(a applier) error {
		if !alg.isValid() {
			return ErrParamAlgorithm
		}

		switch z := a.(type) {
		case *Reader:
			z.p.a = alg
		case *Writer:
			z.p.a = alg

		default:
			return ErrApplyInvalidType
		}

		return nil
	}
}
