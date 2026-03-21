package cli

import "milvus-health/internal/model"

type ExitCodeMapper interface {
	FromAnalysis(result *model.AnalysisResult) int
	FromError(err error) int
}

type DefaultExitCodeMapper struct{}

func (DefaultExitCodeMapper) FromAnalysis(result *model.AnalysisResult) int {
	if result == nil {
		return 4
	}
	switch result.Result {
	case model.FinalResultPASS:
		return 0
	case model.FinalResultWARN:
		return 1
	case model.FinalResultFAIL:
		return 2
	default:
		return 4
	}
}

func (DefaultExitCodeMapper) FromError(err error) int {
	if appErr, ok := err.(*model.AppError); ok {
		switch appErr.Code {
		case model.ErrCodeConfigInvalid:
			return 3
		default:
			return 4
		}
	}
	return 4
}
