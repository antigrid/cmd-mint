package parser

import "cmd-mint/internal/model"

type Result struct {
	Commands []model.CommandRecord
	Summary  model.SourceSummary
}
