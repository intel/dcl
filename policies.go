package dcl

type PolicyParameters struct {
	BufferSize int
	Strategies []StrategyType
	JobParams  JobParams
}

type PolicyFunc func(*PolicyParameters) []StrategyType

func GetDefaultPolicy() PolicyFunc {
	return BufferSizePolicy
}

func BufferSizePolicy(params *PolicyParameters) []StrategyType {
	var list []StrategyType
	if params.BufferSize < 65536 {
		list = []StrategyType{ISAL, IAA, QAT}
	} else {
		list = []StrategyType{QAT, IAA, ISAL}
	}
	return list
}
