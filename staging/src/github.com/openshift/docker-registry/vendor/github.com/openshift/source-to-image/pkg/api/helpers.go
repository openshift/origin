package api

import "time"

// RecordStageAndStepInfo records details about each build stage and step
func RecordStageAndStepInfo(stages []StageInfo, stageName StageName, stepName StepName, startTime time.Time, endTime time.Time) []StageInfo {
	// Make sure that the stages slice is initialized
	if len(stages) == 0 {
		stages = make([]StageInfo, 0)
	}

	// If the stage already exists  update the endTime and Duration, and append the new step.
	for stageKey, stageVal := range stages {
		if stageVal.Name == stageName {
			stages[stageKey].DurationMilliseconds = endTime.Sub(stages[stageKey].StartTime).Nanoseconds() / int64(time.Millisecond)
			if len(stages[stageKey].Steps) == 0 {
				stages[stageKey].Steps = make([]StepInfo, 0)
			}
			stages[stageKey].Steps = append(stages[stageKey].Steps, StepInfo{
				Name:                 stepName,
				StartTime:            startTime,
				DurationMilliseconds: endTime.Sub(startTime).Nanoseconds() / int64(time.Millisecond),
			})
			return stages
		}
	}

	// If the stageName does not exist, add it to the slice along with the new step.
	steps := make([]StepInfo, 0)
	steps = append(steps, StepInfo{
		Name:                 stepName,
		StartTime:            startTime,
		DurationMilliseconds: endTime.Sub(startTime).Nanoseconds() / int64(time.Millisecond),
	})
	stages = append(stages, StageInfo{
		Name:                 stageName,
		StartTime:            startTime,
		DurationMilliseconds: endTime.Sub(startTime).Nanoseconds() / int64(time.Millisecond),
		Steps:                steps,
	})
	return stages
}
