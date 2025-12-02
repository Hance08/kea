package ui

import "github.com/AlecAivazis/survey/v2"

// IconOption returns a survey option that sets the question icon to "-"
// This provides a consistent UI style across all interactive prompts.
func IconOption() survey.AskOpt {
	return survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	})
}
