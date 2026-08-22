package classification

type EffectiveInput struct {
	CurrentAvailability string
	CurrentDNS          string
	CurrentHTTP         string
	CurrentRedirect     string
	CurrentISP          string
	CurrentTLS          string
	CurrentContent      string
	CurrentConfidence   int16
	FailureStreak       int
	SuccessStreak       int
	Observed            Decision
	Qualifying          bool
	OpenFailures        int
	CloseSuccesses      int
}

type EffectiveState struct {
	Availability      string
	DNS               string
	HTTP              string
	Redirect          string
	ISP               string
	TLS               string
	Content           string
	Confidence        int16
	FailureStage      string
	ErrorCode         string
	FailureStreak     int
	SuccessStreak     int
	OpenedIncident    bool
	ClosedIncident    bool
	ChangedDimensions []string
}

func Advance(input EffectiveInput) EffectiveState {
	state := EffectiveState{
		Availability: input.CurrentAvailability, DNS: input.CurrentDNS, HTTP: input.CurrentHTTP,
		Redirect: input.CurrentRedirect, ISP: input.CurrentISP, TLS: input.CurrentTLS, Content: input.CurrentContent,
		Confidence: input.CurrentConfidence, FailureStreak: input.FailureStreak, SuccessStreak: input.SuccessStreak,
		ChangedDimensions: []string{},
	}
	if !input.Qualifying || input.Observed.Availability == "UNKNOWN" {
		return state
	}
	if input.OpenFailures < 1 {
		input.OpenFailures = 3
	}
	if input.CloseSuccesses < 1 {
		input.CloseSuccesses = 2
	}
	previousAvailability := state.Availability
	switch input.Observed.Availability {
	case "ACTIVE":
		state.SuccessStreak++
		state.FailureStreak = 0
		state.FailureStage, state.ErrorCode = "", ""
		switch state.Availability {
		case "UNKNOWN", "ACTIVE":
			state.Availability = "ACTIVE"
		case "DEGRADED", "UNAVAILABLE":
			if state.SuccessStreak >= input.CloseSuccesses {
				state.Availability = "ACTIVE"
			} else {
				state.Availability = "DEGRADED"
			}
		}
	case "DEGRADED", "UNAVAILABLE":
		state.FailureStreak++
		state.SuccessStreak = 0
		state.FailureStage = input.Observed.FailureStage
		state.ErrorCode = input.Observed.ErrorCode
		if state.FailureStreak >= input.OpenFailures {
			state.Availability = "UNAVAILABLE"
		} else {
			state.Availability = "DEGRADED"
		}
	}
	state.DNS = observedOrCurrent(input.Observed.DNS, state.DNS)
	state.HTTP = observedOrCurrent(input.Observed.HTTP, state.HTTP)
	state.Redirect = observedOrCurrent(input.Observed.Redirect, state.Redirect)
	state.ISP = observedOrCurrent(input.Observed.ISP, state.ISP)
	state.TLS = observedOrCurrent(input.Observed.TLS, state.TLS)
	state.Content = observedOrCurrent(input.Observed.Content, state.Content)
	state.Confidence = input.Observed.Confidence
	state.OpenedIncident = previousAvailability != "UNAVAILABLE" && state.Availability == "UNAVAILABLE"
	state.ClosedIncident = previousAvailability == "UNAVAILABLE" && state.Availability == "ACTIVE"
	state.ChangedDimensions = changedDimensions(input, state)
	return state
}

func observedOrCurrent(observed, current string) string {
	if observed == "" || observed == "UNKNOWN" {
		return current
	}
	return observed
}

func changedDimensions(input EffectiveInput, state EffectiveState) []string {
	result := []string{}
	checks := []struct{ name, before, after string }{
		{"availability", input.CurrentAvailability, state.Availability}, {"dns", input.CurrentDNS, state.DNS},
		{"http", input.CurrentHTTP, state.HTTP}, {"redirect", input.CurrentRedirect, state.Redirect},
		{"isp", input.CurrentISP, state.ISP}, {"tls", input.CurrentTLS, state.TLS},
		{"content", input.CurrentContent, state.Content},
	}
	for _, check := range checks {
		if check.before != check.after {
			result = append(result, check.name)
		}
	}
	return result
}
