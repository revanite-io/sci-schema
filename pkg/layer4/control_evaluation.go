package layer4

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// ControlEvaluation is a struct that contains all assessment results, organinzed by name
type ControlEvaluation struct {
	Name              string        // TestSuiteName is the human-readable name or description of the control evaluation
	Control_Id        string        // Control_Id is the unique identifier for the control being evaluated
	Result            Result        // Result is true if all testSets in the testSuite passed
	Message           string        // Message is the human-readable result of the final assessment to run in this evaluation
	Corrupted_State   bool          // BadState is true if any testSet failed to revert at the end of the testSuite
	Remediation_Guide string        // Remediation_Guide is the URL to the documentation for this evaluation
	Assessments       []*Assessment // Control_Evaluations is a map of testSet names to their results
}

func (c *ControlEvaluation) AddAssessment(requirementId string, description string, applicability []string, steps []AssessmentStep) (*Assessment, error) {
	assessment, err := NewAssessment(requirementId, description, applicability, steps)
	if err != nil {
		return nil, err
	}
	c.Assessments = append(c.Assessments, assessment)
	return assessment, nil
}

// Evaluate runs each step in each assessment, updating the relevant fields on the control evaluation.
// It will halt if a step returns a failed result.
// `targetData` is the data that the assessment will be run against.
// `targetApplicability` is a slice of strings that determine when the assessment is applicable.
func (c *ControlEvaluation) Evaluate(targetData interface{}, targetApplicability string) {
	if len(c.Assessments) == 0 {
		c.Result = NeedsReview
		return
	}
	c.closeHandler()
	for _, assessment := range c.Assessments {
		result := assessment.Run(targetData, targetApplicability)
		c.Result = UpdateAggregateResult(c.Result, result)
		if c.Result == Failed {
			break
		}
	}
	c.Cleanup()
}

// TolerantEvaluate runs each step in each assessment, updating the relevant fields on the control evaluation
// It will not halt if a step returns an failed result.
// `targetData` is the data that the assessment will be run against.
// `targetApplicability` is a slice of strings that determine when the assessment is applicable.
func (c *ControlEvaluation) TolerantEvaluate(targetData interface{}, targetApplicability string) {
	if len(c.Assessments) == 0 {
		c.Result = NeedsReview
		return
	}
	c.closeHandler()
	for _, assessment := range c.Assessments {
		result := assessment.RunTolerateFailures(targetData, targetApplicability)
		c.Result = UpdateAggregateResult(c.Result, result)
	}
	c.Cleanup()
}

func (c *ControlEvaluation) Cleanup() {
	for _, assessment := range c.Assessments {
		corrupted := assessment.RevertChanges()
		if corrupted {
			c.Corrupted_State = true
		}
	}
}

// CloseHandler creates a 'listener' on a new goroutine which will notify the
// program if it receives an interrupt from the operating system.
// If an interrupt is received, this will attempt to revert any changes
// made by the terminated ControlEvaluation.
func (c *ControlEvaluation) closeHandler() {
	// Ref: https://golangcode.com/handle-ctrl-c-exit-in-terminal/
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-channel
		log.Print("\n*****\nUnexpected termination. Attempting to revert changes made by the active ControlEvaluation. Do not interrupt this process.\n*****\n")
		c.Cleanup()
		os.Exit(0)
	}()
}
