package analyze

import "cmd-mint/internal/model"

func aliasScore(frequency int, estimatedSavedTotal int, convention bool, confidence model.Confidence, projectSpecific bool) int {
	frequencyWeight := minInt(frequency, 50) * 10
	savingsWeight := minInt(estimatedSavedTotal, 1000) / 10
	conventionBonus := 0
	if convention {
		conventionBonus = 50
	}
	confidenceBonus := 0
	switch confidence {
	case model.ConfidenceHigh:
		confidenceBonus = 30
	case model.ConfidenceMedium:
		confidenceBonus = 10
	}
	specificityPenalty := 0
	if projectSpecific {
		specificityPenalty = 25
	}
	return frequencyWeight + savingsWeight + conventionBonus + confidenceBonus - specificityPenalty
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
